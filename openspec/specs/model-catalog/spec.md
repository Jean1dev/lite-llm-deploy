# model-catalog Specification

## Purpose

Define quais modelos o serviço expõe, a partir da configuração de providers e de um catálogo de preços e metadados embutido, e atende os endpoints de descoberta que os clientes usam para listar modelos.

## Requirements

### Requirement: Providers declarados em configuração

Os providers e suas credenciais MUST ser declarados em configuração do serviço, e não gerenciados em runtime pelo banco. Cada provider declarado MUST informar credencial e, opcionalmente, um endereço base alternativo.

#### Scenario: Provider habilitado por configuração

- **WHEN** o serviço inicia com credenciais declaradas para `openai`, `anthropic` e `gemini`
- **THEN** os modelos desses três providers ficam disponíveis para roteamento e aparecem no catálogo

#### Scenario: Provider sem credencial

- **WHEN** o serviço inicia sem credencial para um provider
- **THEN** os modelos desse provider não aparecem no catálogo e requisições a eles são recusadas

#### Scenario: Falha de inicialização por configuração inválida

- **WHEN** o serviço inicia sem nenhum provider declarado
- **THEN** o serviço recusa iniciar e informa a configuração ausente

### Requirement: Catálogo embutido de modelos

O serviço MUST embarcar um catálogo com os modelos de chat dos providers suportados, contendo, para cada modelo, os preços de entrada e saída por token e os limites de contexto. O catálogo MUST NOT depender de download em runtime.

#### Scenario: Catálogo disponível sem rede

- **WHEN** o serviço inicia sem acesso à internet
- **THEN** o catálogo de modelos está disponível e os endpoints de descoberta respondem normalmente

### Requirement: Expansão de providers em modelos

Cada provider declarado MUST ser expandido nos modelos de chat que o catálogo conhece para aquele provider, reproduzindo o comportamento de curinga do LiteLLM.

#### Scenario: Expansão de um provider

- **WHEN** o provider `anthropic` está declarado
- **THEN** o catálogo exposto contém uma entrada para cada modelo de chat da Anthropic conhecido pelo catálogo, nomeada como `anthropic/<modelo>`

### Requirement: Endpoints de descoberta de modelos

O serviço MUST expor `GET /models` e `GET /v1/models` no formato de lista de modelos da OpenAI, e `GET /model/info` e `GET /v1/model/info` com os metadados do modelo. Os pares com e sem prefixo de versão MUST ter comportamento idêntico.

#### Scenario: Listagem de modelos

- **WHEN** um cliente autenticado chama `GET /v1/models`
- **THEN** a resposta contém `object: "list"` e um item por modelo disponível, com o identificador no formato `<provider>/<modelo>`

#### Scenario: Metadados de modelo

- **WHEN** um cliente autenticado chama `GET /v1/model/info`
- **THEN** cada item traz o nome do modelo e os metadados de preço e limites de contexto correspondentes

### Requirement: Modelo ausente do catálogo

Quando o cliente pedir um modelo cujo provider está declarado mas cujo identificador não consta do catálogo embutido, o serviço MUST encaminhar a requisição ao provider e MUST registrar que o custo não pôde ser calculado, em vez de recusar a chamada.

#### Scenario: Modelo novo lançado pelo provider

- **WHEN** um cliente pede um modelo do provider `openai` que ainda não existe no catálogo embutido
- **THEN** a requisição é atendida pelo provider
- **AND** o serviço registra a ocorrência de custo não calculável para aquela requisição

### Requirement: Origem dos preços usados no custo

O custo de cada requisição MUST ser calculado a partir dos preços do catálogo embutido e do consumo de tokens reportado pelo provider, sem consultar serviço externo.

#### Scenario: Cálculo de custo

- **WHEN** uma requisição a um modelo presente no catálogo é concluída com uso reportado
- **THEN** o custo é o produto dos tokens de entrada e saída pelos respectivos preços do catálogo
