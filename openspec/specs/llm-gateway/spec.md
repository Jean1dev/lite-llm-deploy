# llm-gateway Specification

## Purpose

Recebe requisições de chat completions no formato OpenAI, autoriza o modelo pedido, encaminha para o provider correto e devolve a resposta no mesmo formato que o LiteLLM entrega hoje, de modo que os projetos consumidores não precisem de alteração.

## Requirements

### Requirement: Endpoints de chat completions

O serviço MUST aceitar requisições `POST` em `/chat/completions` e `/v1/chat/completions`, com corpo no formato de chat completions da OpenAI, tanto em modo streaming quanto não-streaming. Os dois caminhos MUST ter comportamento idêntico.

#### Scenario: Requisição não-streaming bem-sucedida

- **WHEN** um cliente autenticado envia `POST /v1/chat/completions` com um `model` autorizado e sem o campo `stream`
- **THEN** o serviço responde `200` com `Content-Type: application/json` e um corpo com `id`, `object: "chat.completion"`, `created`, `model`, `choices` e `usage`

#### Scenario: Caminho sem prefixo de versão

- **WHEN** um cliente envia a mesma requisição para `/chat/completions`
- **THEN** o serviço responde exatamente como responderia em `/v1/chat/completions`

### Requirement: Roteamento por prefixo de provider

O nome do modelo MUST ser interpretado como `<provider>/<modelo>`. O segmento antes da primeira barra determina o provider de destino, e o restante é o identificador enviado ao provider. O serviço MUST suportar os providers `openai`, `anthropic` e `gemini`.

#### Scenario: Modelo roteado para o provider correto

- **WHEN** a requisição informa `"model": "anthropic/claude-haiku-4-5"`
- **THEN** o serviço encaminha a chamada para a API da Anthropic usando `claude-haiku-4-5` como identificador do modelo

#### Scenario: Provider não configurado

- **WHEN** a requisição informa um modelo cujo prefixo não corresponde a nenhum provider configurado
- **THEN** o serviço responde `400` com o envelope de erro padrão e não realiza chamada externa

### Requirement: Preservação de campos desconhecidos da requisição

O serviço MUST encaminhar ao provider os campos do corpo da requisição que não conhece, sem removê-los ou reordená-los, exceto quando uma regra de compatibilidade de parâmetros determinar sua remoção. Campos novos das APIs dos providers MUST funcionar sem alteração no serviço.

#### Scenario: Campo não mapeado é preservado

- **WHEN** a requisição contém um campo que o serviço não conhece e que o provider suporta
- **THEN** o campo chega ao provider com o mesmo nome e valor

### Requirement: Compatibilidade de parâmetros por provider

O serviço MUST remover da requisição os parâmetros que o provider de destino não aceita, em vez de repassá-los e deixar a chamada falhar. O serviço MUST também remover `top_p` nas chamadas a modelos Anthropic, mesmo sendo um parâmetro suportado, preservando `temperature`.

#### Scenario: Parâmetro não suportado pelo provider

- **WHEN** um cliente OpenAI envia `presence_penalty` e `frequency_penalty` para um modelo Anthropic
- **THEN** o serviço remove os dois parâmetros antes de chamar o provider e a requisição é atendida normalmente

#### Scenario: Sampling conflitante em modelos Anthropic

- **WHEN** um cliente envia `temperature` e `top_p` para um modelo Anthropic
- **THEN** o serviço remove `top_p`, mantém `temperature` e a requisição é atendida normalmente

### Requirement: Formato da resposta não-streaming

A resposta MUST seguir o formato de chat completion da OpenAI. O campo `model` MUST conter o nome do modelo exatamente como o cliente o enviou, incluindo o prefixo do provider, e não o identificador interno do provider. Quando o provider não fornecer um identificador compatível, o serviço MUST gerar um `id` no formato `chatcmpl-<uuid>`.

#### Scenario: Campo model reflete o pedido do cliente

- **WHEN** o cliente envia `"model": "anthropic/claude-haiku-4-5"` e o provider responde com sucesso
- **THEN** a resposta contém `"model": "anthropic/claude-haiku-4-5"`

#### Scenario: Identificador gerado para provider sem id compatível

- **WHEN** o provider de destino devolve um identificador em formato próprio, como o `msg_...` da Anthropic
- **THEN** a resposta ao cliente traz um `id` no formato `chatcmpl-<uuid>`

#### Scenario: Uso reportado na resposta

- **WHEN** a chamada não-streaming é concluída com sucesso
- **THEN** a resposta contém `usage` com `prompt_tokens`, `completion_tokens` e `total_tokens` correspondentes ao que o provider reportou

### Requirement: Streaming em Server-Sent Events

Em modo streaming, o serviço MUST responder com `Content-Type: text/event-stream`, emitir cada chunk como uma linha `data: ` com um objeto `chat.completion.chunk`, e encerrar com `data: [DONE]`. Os chunks MUST ser encaminhados ao cliente à medida que chegam do provider, sem aguardar o fim da resposta.

#### Scenario: Sequência de chunks

- **WHEN** um cliente envia `"stream": true`
- **THEN** o serviço emite um ou mais chunks com `delta`, um chunk final com `finish_reason` e, por último, `data: [DONE]`

#### Scenario: Encaminhamento incremental

- **WHEN** o provider emite um chunk
- **THEN** o serviço repassa esse chunk ao cliente antes de receber o chunk seguinte

### Requirement: Contabilização de uso em streaming

O serviço MUST obter o consumo de tokens de respostas em streaming a partir dos dados do próprio provider, sem estimar por contagem local de tokens. Quando for necessário solicitar esse dado ao provider, o serviço MUST fazê-lo sem alterar o fluxo de eventos entregue ao cliente.

#### Scenario: Cliente não solicita usage

- **WHEN** um cliente envia `"stream": true` sem `stream_options.include_usage`
- **THEN** o fluxo entregue ao cliente não contém chunk de `usage`
- **AND** o serviço registra o consumo real de tokens da requisição

#### Scenario: Cliente solicita usage

- **WHEN** um cliente envia `stream_options.include_usage` igual a `true`
- **THEN** o fluxo entregue ao cliente contém o chunk final com `usage`

### Requirement: Headers de resposta

O serviço MUST incluir nas respostas os headers de diagnóstico equivalentes aos do LiteLLM, entre eles o identificador da chamada, o grupo e o nome do modelo, o identificador do registro de modelo, o custo da resposta e o spend acumulado da chave. O serviço MUST também repassar os headers recebidos do provider prefixados com `llm_provider-`.

#### Scenario: Headers de diagnóstico presentes

- **WHEN** uma chamada não-streaming é concluída com sucesso
- **THEN** a resposta inclui `x-litellm-call-id`, `x-litellm-model-group`, `x-litellm-model-name`, `x-litellm-model-id`, `x-litellm-response-cost` e `x-litellm-key-spend`

#### Scenario: Headers do provider espelhados

- **WHEN** o provider retorna headers de rate limit
- **THEN** esses headers aparecem na resposta ao cliente com o prefixo `llm_provider-`

#### Scenario: Custo desconhecido em streaming

- **WHEN** a resposta é em streaming e os headers são emitidos antes do fim do fluxo
- **THEN** os headers de custo são emitidos com valor zero, como ocorre hoje

### Requirement: Envelope de erro

Toda resposta de erro MUST usar o envelope `{"error": {"message", "type", "param", "code"}}`, com `code` em formato de texto. Erros originados no provider MUST preservar a mensagem e o código de status recebidos.

#### Scenario: Falha de autenticação

- **WHEN** a requisição chega sem chave válida
- **THEN** o serviço responde `401` com um corpo no envelope de erro e `type` indicando a natureza da falha

#### Scenario: Erro repassado do provider

- **WHEN** o provider responde com erro
- **THEN** o serviço responde com o mesmo código de status e uma mensagem que preserva a descrição do provider

### Requirement: Consumo de memória por requisição

O serviço MUST NOT acumular em memória o corpo completo das respostas em streaming. A memória usada por requisição em streaming MUST ser limitada e independente do tamanho total da resposta gerada.

#### Scenario: Resposta longa em streaming

- **WHEN** uma requisição em streaming produz uma resposta de tamanho muito superior ao de um chunk
- **THEN** o consumo de memória atribuível à requisição permanece limitado ao tamanho dos buffers de encaminhamento
