## Why

O LiteLLM em produção consome memória muito acima do que a operação justifica, e o custo de hospedagem cresceu junto. A instalação atual roda a versão 1.100.0 expondo 606 rotas e 817 operações, mas o uso real depende de três linhas de modelo no banco (`openai/*`, `anthropic/*`, `gemini/*`) e de um punhado de endpoints. Estamos pagando a memória de uma plataforma inteira para usar cerca de 2% dela.

A latência não é o problema: o overhead medido do proxy é de 6,2 a 6,5 ms por requisição. O objetivo desta mudança é exclusivamente reduzir footprint de memória e custo de infraestrutura, mantendo o contrato de API intacto para que nenhum projeto consumidor precise de alteração de código.

## What Changes

- Novo serviço `litellm-diet`: um único binário Go que substitui o proxy LiteLLM no caminho de produção.
- Gateway de inferência limitado a `POST /chat/completions` e `POST /v1/chat/completions`, com e sem streaming, reproduzindo byte a byte o contrato atual, incluindo os headers `x-litellm-*` e o espelhamento de headers do provider com prefixo `llm_provider-`.
- Roteamento por prefixo do nome do modelo para três providers, sem router, sem load balancing, sem fallback e sem retry entre deployments.
- Tradução própria sobre as APIs nativas de Anthropic e Gemini, mantendo a fidelidade do formato OpenAI produzido hoje pelo LiteLLM. OpenAI permanece pass-through.
- Virtual keys com autenticação por `sha256` da chave, mantendo o mesmo esquema de hash do LiteLLM para que as chaves já distribuídas continuem válidas.
- Spend, budget e last active por chave, com contabilização agregada em memória e escrita em lote no banco.
- Catálogo de modelos servido a partir de um mapa de preços e metadados embutido no binário, reproduzindo a expansão wildcard atual (340 entradas a partir de 3 registros).
- Schema de banco próprio e enxuto, com script de migração das chaves existentes a partir do banco do LiteLLM.
- **BREAKING** para operação, não para consumidores: a interface de administração web deixa de existir; gestão de chaves passa a ser exclusivamente por API.
- **BREAKING** para operação: os modelos deixam de ser gerenciáveis em runtime pelo banco e passam a ser declarados em configuração; adicionar provider exige deploy.
- Removidos do escopo: teams, organizations, users, MCP, guardrails, agents, cache de resposta em Redis, rotação automática de chaves, tags, audit log e todos os endpoints de inferência fora de chat completions.

## Capabilities

### New Capabilities

- `llm-gateway`: recepção de chat completions, autorização de modelo, roteamento por prefixo para OpenAI, Anthropic e Gemini, tradução de requisição e resposta entre o formato OpenAI e as APIs nativas, streaming SSE, captura de uso e composição dos headers de resposta.
- `virtual-keys`: autenticação por hash de chave, ciclo de vida das chaves (criação, atualização, remoção, bloqueio e desbloqueio), consulta e listagem, e os campos de spend, budget com janela de reset e last active.
- `model-catalog`: catálogo de modelos derivado da configuração de providers e do mapa de preços embutido, expansão wildcard e os endpoints de descoberta consumidos pelos clientes.
- `key-migration`: importação das chaves existentes do banco do LiteLLM para o schema próprio, preservando hash, spend acumulado, limites de budget, janela de reset e last active.

### Modified Capabilities

Nenhuma. O repositório não possui specs em `openspec/specs/`.

## Impact

- **Novo repositório/serviço**: código Go do `litellm-diet`. Este repositório hoje contém apenas artefatos de deploy (`Dockerfile`, `docker-compose.yml`, `config.yaml`).
- **API**: contrato preservado para os consumidores. O risco concentra-se na fidelidade da tradução de Anthropic e Gemini, que hoje é feita pelo LiteLLM.
- **Banco de dados**: novo schema, com migração unidirecional a partir do banco atual. As chaves em texto plano não são recuperáveis do banco (apenas o hash é armazenado), o que torna obrigatório manter o esquema `sha256`.
- **Infraestrutura**: o serviço Redis deixa de ser necessário no caminho de dados. O deploy na Railway passa a construir um binário Go em vez de estender a imagem oficial do LiteLLM.
- **Credenciais**: as três credenciais de provider passam a ser configuração do serviço. O `LITELLM_SALT_KEY` deixa de ser necessário em runtime, sendo usado apenas se a migração optar por extrair as credenciais do banco atual.
- **Operação**: perda da interface web e dos relatórios agregados de spend do LiteLLM. Perda do histórico detalhado de `LiteLLM_SpendLogs`, salvo se migrado separadamente.
- **Manutenção**: a compatibilidade com mudanças das APIs dos três providers passa a ser responsabilidade do time.
