## 1. Fundação do projeto

- [x] 1.1 Criar o repositório do serviço com módulo Go, layout de diretórios e `Makefile`, verificando que `go build ./...` e `go vet ./...` passam em repositório limpo
- [x] 1.2 Configurar formatação, linter e execução de testes na integração contínua, verificando que um commit com código mal formatado é reprovado
- [x] 1.3 Definir o teto de memória do runtime por configuração e expor o valor efetivo no log de inicialização, verificando que o serviço sobe com o teto aplicado
- [x] 1.4 Criar `Dockerfile` de produção com binário estático e imagem mínima, verificando que o contêiner sobe e responde ao endpoint de liveness
- [x] 1.5 Criar `docker-compose.yaml` de desenvolvimento com o serviço e Postgres, verificando que `docker compose up` deixa o ambiente pronto sem dependências instaladas na máquina

## 2. Configuração e catálogo de modelos

- [x] 2.1 Implementar o carregamento da configuração de providers com credencial e endereço base opcional, verificando por teste que a ausência total de providers impede a inicialização
- [x] 2.2 Implementar o gerador de build que produz o catálogo podado a partir do mapa de preços do LiteLLM, restrito aos modelos de chat dos três providers, verificando que o artefato gerado contém preços e limites de contexto para cada entrada
- [x] 2.3 Embutir o catálogo no binário e implementar a expansão de cada provider declarado em seus modelos, verificando por teste que a contagem de entradas por provider corresponde ao catálogo
- [x] 2.4 Implementar a resolução de preço por modelo e o cálculo de custo a partir do uso reportado, verificando por teste de tabela os casos de modelo conhecido e de modelo ausente do catálogo

## 3. Persistência e estado em memória

- [x] 3.1 Definir o schema da tabela de chaves com o hash como chave primária e criar a migração inicial, verificando que a migração aplica e reverte em banco vazio
- [x] 3.2 Implementar o carregamento das chaves para o mapa em memória e sua atualização periódica, verificando por teste de integração que uma alteração no banco passa a valer após o intervalo de atualização
- [x] 3.3 Implementar o agregador em memória de gasto e último uso com gravação em lote, verificando por teste de integração que N requisições da mesma chave produzem uma única escrita agregada
- [x] 3.4 Implementar a gravação forçada do agregado no encerramento gracioso, verificando por teste que nenhum gasto pendente é perdido ao encerrar o processo

## 4. Autenticação e autorização

- [x] 4.1 Implementar a autenticação por `sha256` da chave apresentada no header de autorização, verificando por teste que o hash gerado para uma chave conhecida corresponde ao valor produzido pelo LiteLLM
- [x] 4.2 Implementar a recusa de chaves bloqueadas, expiradas e com orçamento esgotado, verificando por teste de tabela cada estado e o código de resposta correspondente
- [x] 4.3 Implementar a autorização de modelo por chave com suporte a curinga por provider, verificando por teste os casos de lista vazia, curinga correspondente e modelo fora da lista
- [x] 4.4 Implementar a autorização administrativa por chave mestra nos endpoints de gestão, verificando por teste que uma chave comum é recusada

## 5. Gateway de chat completions

- [x] 5.1 Implementar o servidor HTTP e o roteamento dos endpoints declarados, verificando por teste que os caminhos com e sem prefixo de versão respondem de forma idêntica
- [x] 5.2 Implementar a extração do campo de modelo do corpo sem desserializar a requisição inteira, verificando por benchmark que a alocação por requisição não cresce com o tamanho do prompt
- [x] 5.3 Implementar o roteamento por prefixo de provider e a recusa de prefixo não configurado, verificando por teste os três providers e o caso de prefixo desconhecido
- [x] 5.4 Implementar o filtro de parâmetros por provider, incluindo a remoção de `top_p` em modelos Anthropic, verificando por teste que os parâmetros esperados são removidos e os demais preservados
- [x] 5.5 Implementar o envelope de erro padrão e o repasse de erros do provider, verificando por teste que o formato e o código de status correspondem ao contrato
- [x] 5.6 Implementar a composição dos headers de resposta, incluindo os de diagnóstico e o espelhamento dos headers do provider com prefixo, verificando por teste a presença e o valor de cada header
- [x] 5.7 Implementar o encaminhamento não-streaming para OpenAI sem re-serializar o corpo, verificando por teste de integração que a resposta preserva campos desconhecidos
- [x] 5.8 Implementar o encaminhamento em streaming com repasse incremental de chunks e terminação com `[DONE]`, verificando por teste que o primeiro chunk chega ao cliente antes do fim da resposta do provider
- [x] 5.9 Implementar a solicitação de uso ao provider em streaming e a supressão do chunk correspondente quando o cliente não o pediu, verificando por teste os dois casos de `stream_options`
- [x] 5.10 Verificar por teste de carga em streaming que a memória residente permanece estável e independente do tamanho total das respostas geradas

## 6. Tradução Anthropic

- [x] 6.1 Implementar a conversão de requisição do formato OpenAI para a API nativa da Anthropic, cobrindo mensagens, mensagem de sistema, parâmetros de sampling e limite de tokens, verificando por teste de tabela
- [x] 6.2 Implementar a conversão de resposta não-streaming para o formato OpenAI, incluindo geração do identificador, mapeamento de motivo de término, uso e `provider_specific_fields`, verificando por teste contra respostas capturadas
- [x] 6.3 Implementar a conversão do fluxo de eventos nativo para chunks no formato OpenAI, verificando por teste que a sequência de chunks corresponde à capturada do proxy atual
- [x] 6.4 Implementar a conversão de chamadas de ferramenta nos dois sentidos, verificando por teste de integração uma conversa completa com uso de ferramenta
- [x] 6.5 Implementar a conversão de conteúdo multimodal em mensagens de entrada, verificando por teste uma requisição com imagem

## 7. Tradução Gemini

- [x] 7.1 Implementar a conversão de requisição do formato OpenAI para a API nativa do Gemini, verificando por teste de tabela
- [x] 7.2 Implementar a conversão de resposta não-streaming para o formato OpenAI, incluindo uso e mapeamento de motivo de término, verificando por teste contra respostas capturadas
- [x] 7.3 Implementar a conversão do fluxo de eventos nativo para chunks no formato OpenAI, verificando por teste a sequência de chunks
- [x] 7.4 Implementar a conversão de chamadas de ferramenta nos dois sentidos, verificando por teste de integração uma conversa completa com uso de ferramenta

## 8. Gasto, orçamento e último uso

- [x] 8.1 Integrar o cálculo de custo ao término de cada requisição atendida, verificando por teste de integração que o gasto da chave reflete o uso reportado
- [x] 8.2 Implementar o registro de custo não calculável para modelo ausente do catálogo, verificando por teste que a requisição é atendida e a ocorrência registrada
- [x] 8.3 Implementar a janela de orçamento com reset por duração configurada, verificando por teste os casos de janela vencida, janela em andamento e chave sem duração
- [x] 8.4 Implementar o bloqueio por orçamento esgotado no caminho de autorização, verificando por teste que a requisição seguinte ao esgotamento é recusada sem chamada externa
- [x] 8.5 Integrar a atualização de último uso ao caminho autenticado, verificando por teste de integração que o valor persiste após a gravação em lote

## 9. Endpoints de gestão de chaves

- [x] 9.1 Implementar a geração de chave no formato esperado, retornando o texto plano apenas na resposta de criação, verificando por teste o formato e a ausência do texto plano nas consultas
- [x] 9.2 Implementar atualização, remoção, bloqueio e desbloqueio, verificando por teste de integração o efeito de cada operação na autorização subsequente
- [x] 9.3 Implementar consulta individual e listagem expondo gasto, orçamento, janela e último uso, verificando por teste a presença de todos os campos declarados
- [ ] 9.4 Verificar por teste de contrato que os corpos de requisição e resposta dos endpoints de gestão correspondem aos do proxy atual

## 10. Descoberta de modelos e saúde

- [x] 10.1 Implementar os endpoints de listagem de modelos com resposta pré-serializada, verificando por teste que o corpo é idêntico entre chamadas e que os caminhos com e sem prefixo de versão coincidem
- [x] 10.2 Implementar os endpoints de metadados de modelo com resposta pré-serializada, verificando por teste a presença de preços e limites de contexto
- [x] 10.3 Implementar os endpoints de liveness e readiness, verificando que readiness reflete o estado da conexão com o banco

## 11. Migração de chaves

- [x] 11.1 Implementar a leitura das chaves do banco de origem com filtro das chaves removidas, verificando por teste de integração contra uma cópia do schema do LiteLLM
- [x] 11.2 Implementar a escrita idempotente no banco de destino preservando os campos declarados, verificando por teste que duas execuções consecutivas produzem o mesmo estado
- [x] 11.3 Implementar o modo de verificação sem escrita e o relatório de execução com contagens e motivos, verificando por teste que o destino permanece inalterado no modo de verificação
- [x] 11.4 Implementar a decifragem opcional das credenciais de provider nos dois formatos usados pelo LiteLLM, verificando por teste com valores cifrados de cada formato
- [ ] 11.5 Executar a migração em modo de verificação contra o banco de produção e revisar o relatório com o time

## 12. Validação de compatibilidade

- [x] 12.1 Implementar o harness que envia a mesma requisição aos dois serviços e compara corpo e headers, com lista explícita de campos que podem divergir, verificando que uma divergência injetada é detectada
- [x] 12.2 Estender o harness para comparar a sequência de chunks em streaming, verificando que uma diferença de ordem ou de conteúdo é detectada
- [ ] 12.3 Levantar no banco atual a distribuição de chamadas por endpoint, provider e modelo, e o pico de requisições por minuto, verificando que o escopo declarado cobre o uso real e que uma réplica é suficiente
- [ ] 12.4 Executar o harness sobre um conjunto representativo de requisições reais, com contabilização de gasto desligada, e registrar as divergências remanescentes com justificativa

## 13. Deploy e cutover

- [ ] 13.1 Publicar o serviço na Railway em paralelo ao LiteLLM, sem tráfego de produção, verificando que readiness responde saudável com o banco próprio
- [ ] 13.2 Executar a migração de chaves para valer e conferir por amostragem que chaves existentes autenticam no serviço novo
- [ ] 13.3 Redirecionar o endereço consumido pelos projetos para o serviço novo e acompanhar taxa de erro e gasto na primeira janela de operação
- [ ] 13.4 Comparar o consumo de memória e o custo de hospedagem antes e depois, registrando o resultado como fechamento da motivação da mudança
- [ ] 13.5 Manter o LiteLLM e seu banco desligados do tráfego pelo período de retenção definido, verificando que o rollback por reapontamento de endereço permanece viável
