## Context

Ver `proposal.md` para a motivação. O que importa aqui é o que foi levantado da instalação atual, porque restringe fortemente o desenho.

A instalação roda LiteLLM 1.100.0 na Railway, com Postgres e Redis. O banco contém **três** registros de modelo (`openai/*`, `anthropic/*`, `gemini/*`), confirmados pela contagem de identificadores distintos em `/model/info`: três UUIDs, repetidos 223, 89 e 28 vezes, totalizando 340 entradas expostas. Não há roteamento entre deployments, alias, fallback ou retry configurados.

O contrato observado nas respostas de produção:

- O campo `model` da resposta repete o nome pedido pelo cliente, com prefixo de provider.
- O `id` é pass-through no OpenAI e gerado como `chatcmpl-<uuid>` no Anthropic.
- O LiteLLM injeta `provider_specific_fields` em `message` e `choices`, com conteúdo diferente por provider.
- São emitidos cerca de 25 headers `x-litellm-*` mais o espelhamento de todos os headers do provider com prefixo `llm_provider-`.
- Em streaming os headers de custo saem zerados, porque são emitidos antes do fim do fluxo.
- Em streaming sem `stream_options.include_usage`, nenhum chunk de `usage` é entregue ao cliente.
- O overhead próprio do proxy é de 6,2 a 6,5 ms.

Sobre as chaves: a coluna `token` da `LiteLLM_VerificationToken` é o `sha256` hexadecimal da string completa da chave, sem salt e sem HMAC. Isso foi verificado reproduzindo localmente o hash que o proxy reportou em uma falha de autenticação. As chaves em texto plano não existem no banco.

O `LITELLM_SALT_KEY` cifra apenas os `litellm_params` da tabela de modelos, com chave derivada por `sha256` simples. Há dois formatos: NaCl SecretBox (XSalsa20-Poly1305), que é o default legado e não tem marcador, e AES-256-GCM identificado pelo prefixo `v2:gcm:`.

## Goals / Non-Goals

**Goals:**

- Consumo de memória estável e limitado por teto explícito, independente do tamanho das respostas geradas.
- Compatibilidade de contrato verificável por diferença contra o proxy atual, e não por inspeção manual.
- Superfície de código pequena o suficiente para ser mantida pelo time sem dedicação contínua.

**Non-Goals:**

- Ganho de latência. O overhead atual já é baixo; qualquer melhora é efeito colateral, não objetivo.
- Histórico de uso por requisição equivalente a `LiteLLM_SpendLogs`.
- Operação com múltiplas réplicas ativas na primeira versão.
- Paridade com o LiteLLM fora dos endpoints declarados nas specs.

## Decisions

### Binário único em Go, sem CGO, com teto de memória explícito

Um processo, sem runtime externo, sem engine de banco separado. O teto de memória é declarado ao runtime, o que dá um comportamento previsível sob pressão em vez de crescimento até o limite do container.

Alternativa considerada: ajustar o LiteLLM atual (reduzir workers, desabilitar imports de provider, fixar versão). Rejeitada porque o piso de memória do Python com os SDKs dos providers e o engine do Prisma permanece alto mesmo com um worker, e porque não elimina o custo de manter uma plataforma de 606 rotas para usar 16.

### Encaminhamento por bytes no caminho OpenAI, tradução apenas onde é obrigatória

Requisições e respostas de modelos OpenAI são encaminhadas sem serem convertidas para estruturas intermediárias: extrai-se apenas o campo `model` para roteamento e autorização, e o restante do corpo segue como recebido. Isso preserva campos que o serviço não conhece e evita alocação proporcional ao tamanho do prompt.

Anthropic e Gemini exigem conversão entre o formato OpenAI e as APIs nativas, nos dois sentidos e também no fluxo de eventos. Essa conversão é o maior bloco de trabalho do projeto e fica isolada atrás de uma fronteira por provider, para que o caminho OpenAI não pague por ela.

Alternativa considerada: usar os endpoints OpenAI-compatíveis publicados por Anthropic e Google, o que tornaria o serviço inteiro pass-through. Rejeitada nesta versão porque esses endpoints não expõem toda a superfície das APIs nativas, e o objetivo declarado é fidelidade ao comportamento atual.

### Consumo de tokens obtido do provider, nunca estimado

Contagem local de tokens exigiria embarcar tabelas de encoding, que é justamente um dos custos de memória que se quer eliminar. Em não-streaming o uso vem no corpo da resposta. Em streaming, Anthropic e Gemini já emitem uso no fluxo nativo; para OpenAI, o serviço solicita o uso ao provider e suprime o chunk correspondente antes de entregar ao cliente, a menos que o cliente o tenha pedido.

Consequência aceita: se o provider não devolver uso, a requisição é registrada como custo não calculável em vez de estimada.

Alternativa considerada: sempre repassar o chunk de uso ao cliente. Rejeitada porque altera os bytes que os consumidores recebem hoje.

### Estado quente em memória, persistência em lote

As chaves ficam em um mapa indexado pelo hash, atualizado periodicamente, de modo que a autenticação não toca o banco. Gasto e último uso são agregados por chave em memória e gravados em lote. O banco deixa de receber escrita por requisição.

Isso implica que o bloqueio por orçamento opera sobre um valor que pode estar alguns segundos atrasado, o que foi aceito explicitamente. Com uma réplica, o contador em memória é a fonte mais atual, e o atraso afeta apenas a durabilidade, não a decisão.

Alternativa considerada: escrita síncrona antes de liberar a requisição. Rejeitada pelo custo de latência e de carga no banco para um ganho de precisão da ordem de centavos.

### Catálogo embutido no binário e respostas pré-serializadas

O catálogo de modelos é gerado em tempo de build a partir do mapa de preços do LiteLLM, podado para os modelos de chat dos três providers. As respostas dos endpoints de descoberta são serializadas uma vez na inicialização e servidas a partir de um buffer imutável, porque são grandes (a resposta equivalente hoje passa de 1,8 MB) e não mudam entre requisições.

Alternativa considerada: buscar o mapa em runtime a partir do repositório do LiteLLM. Rejeitada por criar dependência de rede na inicialização e tornar o comportamento do serviço dependente de um terceiro.

### Schema próprio, com o hash como chave primária

O schema é novo e enxuto, mas a chave primária da tabela de chaves é obrigatoriamente o `sha256` hexadecimal usado pelo LiteLLM. Não é uma escolha de compatibilidade cosmética: como as chaves em texto plano não existem no banco de origem, qualquer outro esquema de identificação exigiria reemitir chave para todos os consumidores, que é exatamente o que a mudança quer evitar.

### Uma réplica, sem Redis

Sem estado compartilhado, contadores em processo são exatos e o Redis sai do caminho de dados. O desenho assume uma réplica ativa.

Alternativa considerada: contadores em Redis desde o início. Rejeitada por adicionar dependência e custo antes de haver evidência de que o volume exige mais de uma réplica. Escalar horizontalmente exige revisitar esta decisão.

### Compatibilidade validada por diferença contra o proxy atual

"Igual ao LiteLLM" não é verificável por leitura. O critério de aceite é um harness que envia a mesma requisição aos dois serviços e compara corpo e headers, com lista explícita de campos que podem divergir legitimamente, como identificadores e marcas de tempo. O mesmo harness cobre o fluxo de eventos em streaming, comparando a sequência de chunks.

O harness roda contra o serviço novo sem contabilizar gasto, para não poluir os totais durante a validação.

## Risks / Trade-offs

- **Endpoint em uso fora do escopo declarado** → O inventário de uso real ainda não foi levantado; `/embeddings` e `/responses` estão expostos hoje. Levantar a distribuição de chamadas por endpoint no banco atual antes do cutover e, se houver uso, tratar como escopo adicional em vez de descobrir em produção.
- **Divergência de comportamento na tradução de Anthropic e Gemini** → É o maior risco técnico, concentrado em tool calls, conteúdo multimodal, blocos de raciocínio e no fluxo de eventos. Mitigado pelo harness de diferença executado sobre tráfego representativo, com o cutover condicionado ao resultado.
- **Esteira de manutenção dos providers** → Cada quirk de API passa a ser responsabilidade do time. Mitigado mantendo o conjunto de providers pequeno e o caminho OpenAI como pass-through, para que mudanças naquela API não exijam alteração de código.
- **Modelo novo lançado por um provider não consta do catálogo embutido** → A requisição é atendida, mas o custo não é calculado. Mitigado por registro explícito da ocorrência e por rotina de atualização do catálogo.
- **Perda do histórico por requisição** → Sem equivalente a `LiteLLM_SpendLogs`, não há como auditar um gasto específico depois. O banco antigo deve ser preservado como arquivo histórico após o cutover.
- **Divergência de gasto durante execução em paralelo** → Se os dois proxies atenderem tráfego real ao mesmo tempo, cada um contabiliza no seu banco. Evitado fazendo o espelhamento sem contabilização e o cutover de uma vez.
- **Perda de gasto em encerramento não gracioso** → A janela é o intervalo entre gravações em lote. Aceito, com gravação forçada no encerramento limpo e intervalo curto o suficiente para que a perda máxima seja irrelevante frente ao orçamento.

## Migration Plan

1. Subir o serviço novo em paralelo, sem tráfego de produção, apontando para um banco próprio vazio.
2. Levantar no banco atual a distribuição de chamadas por endpoint, provider e modelo, e o pico de requisições por minuto. Confirmar que o escopo declarado cobre o uso real e que uma réplica é suficiente.
3. Executar o harness de diferença sobre tráfego representativo, sem contabilizar gasto, até que as divergências restantes estejam na lista aceita.
4. Executar a migração de chaves em modo de verificação e conferir o relatório.
5. Executar a migração de chaves para valer, com o proxy antigo ainda atendendo.
6. Cutover: apontar o endereço consumido pelos projetos para o serviço novo, de uma vez.
7. Manter o LiteLLM e seu banco intactos e desligados do tráfego por um período de retenção definido.

**Rollback**: reapontar o endereço para o LiteLLM, que permanece funcional com seu próprio banco. O gasto contabilizado pelo serviço novo durante o período não é transferido de volta; a reconciliação, se necessária, é manual a partir do relatório de gasto do período.

## Open Questions

- Frequência e gatilho da atualização do catálogo embutido: em cada release, agendada, ou disparada pela detecção de modelo desconhecido.
- Conteúdo exato de `provider_specific_fields` nos cenários de tool call e de blocos de raciocínio. Resolvido pelo harness de diferença durante a implementação, sem alterar as specs.
- Período de retenção do banco do LiteLLM como arquivo histórico após o cutover.
