# virtual-keys Specification

## Purpose

Autentica as chamadas ao gateway por meio de chaves virtuais e mantém, para cada chave, o gasto acumulado, o limite de orçamento com janela de reset e o momento do último uso.

## Requirements

### Requirement: Autenticação por hash da chave

O serviço MUST autenticar requisições pelo header `Authorization: Bearer <chave>` e identificar a chave pelo `sha256` hexadecimal da string completa da chave, sem salt e sem HMAC. O serviço MUST NOT armazenar a chave em texto plano.

#### Scenario: Chave válida

- **WHEN** uma requisição apresenta uma chave cujo hash existe e está ativa
- **THEN** a requisição é autorizada e associada a essa chave

#### Scenario: Chave inexistente

- **WHEN** uma requisição apresenta uma chave cujo hash não existe
- **THEN** o serviço responde `401` sem consultar o provider

#### Scenario: Chave emitida pelo LiteLLM

- **WHEN** uma chave gerada pelo LiteLLM antes da migração é apresentada
- **THEN** a autenticação é bem-sucedida sem que o cliente precise trocar de chave

### Requirement: Estados que impedem o uso da chave

O serviço MUST recusar requisições de chaves bloqueadas, expiradas ou que tenham atingido o limite de orçamento da janela corrente.

#### Scenario: Chave bloqueada

- **WHEN** uma chave com bloqueio ativo é usada
- **THEN** o serviço responde `401` e não realiza chamada externa

#### Scenario: Chave expirada

- **WHEN** uma chave cuja data de expiração já passou é usada
- **THEN** o serviço responde `401` e não realiza chamada externa

#### Scenario: Orçamento esgotado

- **WHEN** o gasto acumulado da chave na janela corrente atinge ou ultrapassa o limite de orçamento
- **THEN** o serviço recusa novas requisições dessa chave até o reset da janela

### Requirement: Autorização de modelo por chave

Quando uma chave declara uma lista de modelos permitidos, o serviço MUST recusar requisições a modelos fora dessa lista. A lista MUST aceitar entradas curinga no formato `<provider>/*`. Uma lista vazia significa acesso a todos os modelos configurados.

#### Scenario: Modelo permitido por curinga

- **WHEN** a chave permite `openai/*` e a requisição pede `openai/gpt-4o-mini`
- **THEN** a requisição é autorizada

#### Scenario: Modelo fora da lista

- **WHEN** a chave permite apenas `openai/*` e a requisição pede `anthropic/claude-haiku-4-5`
- **THEN** o serviço recusa a requisição e não realiza chamada externa

### Requirement: Geração de chaves

O serviço MUST expor `POST /key/generate` para criar chaves. A chave gerada MUST seguir o formato `sk-` seguido de 22 caracteres em base64 url-safe e MUST ser retornada em texto plano apenas nessa resposta. A requisição MUST aceitar ao menos `key_alias`, `models`, `max_budget`, `budget_duration` e `duration`.

#### Scenario: Criação bem-sucedida

- **WHEN** um administrador chama `POST /key/generate` com um corpo válido
- **THEN** o serviço responde com a chave em texto plano e os atributos atribuídos a ela

#### Scenario: Chave não recuperável depois

- **WHEN** qualquer endpoint de consulta é chamado para uma chave existente
- **THEN** a resposta não contém a chave em texto plano

### Requirement: Ciclo de vida das chaves

O serviço MUST expor endpoints para atualizar, remover, bloquear e desbloquear chaves, e para consultar uma chave individualmente e listar chaves.

#### Scenario: Atualização de atributos

- **WHEN** `POST /key/update` é chamado alterando `max_budget`
- **THEN** o novo limite passa a valer para as requisições seguintes da chave

#### Scenario: Bloqueio e desbloqueio

- **WHEN** `POST /key/block` é chamado para uma chave e em seguida `POST /key/unblock`
- **THEN** a chave passa a recusar requisições após o bloqueio e volta a aceitá-las após o desbloqueio

#### Scenario: Remoção

- **WHEN** `POST /key/delete` é chamado para uma chave
- **THEN** requisições subsequentes com essa chave são recusadas com `401`

### Requirement: Consulta de spend, budget e last active

Os endpoints `GET /key/info` e `GET /key/list` MUST expor, para cada chave, o gasto acumulado, o limite de orçamento, a duração e o instante do próximo reset da janela, e o instante do último uso.

#### Scenario: Consulta individual

- **WHEN** `GET /key/info` é chamado para uma chave existente
- **THEN** a resposta contém `spend`, `max_budget`, `budget_duration`, `budget_reset_at` e `last_active`

#### Scenario: Listagem

- **WHEN** `GET /key/list` é chamado
- **THEN** cada item da lista contém os mesmos campos de gasto, orçamento e último uso

### Requirement: Contabilização de gasto

O serviço MUST somar ao gasto da chave o custo de cada requisição atendida com sucesso, calculado a partir do consumo de tokens reportado pelo provider e dos preços do catálogo de modelos. A contabilização MUST ser agregada em memória e persistida em lote, sem uma escrita no banco por requisição.

#### Scenario: Gasto somado após a requisição

- **WHEN** uma requisição é atendida com sucesso
- **THEN** o custo correspondente passa a compor o gasto acumulado da chave

#### Scenario: Persistência em lote

- **WHEN** várias requisições da mesma chave são atendidas em sequência
- **THEN** o banco recebe uma escrita agregada em vez de uma escrita por requisição

#### Scenario: Perda tolerada em encerramento abrupto

- **WHEN** o processo é encerrado de forma limpa
- **THEN** o gasto acumulado ainda não persistido é gravado antes do encerramento

### Requirement: Janela de orçamento

Quando a chave define uma duração de orçamento, o serviço MUST zerar o gasto da janela e avançar o instante de reset assim que o instante corrente ultrapassar o reset programado.

#### Scenario: Reset da janela

- **WHEN** o instante de reset de uma chave é ultrapassado
- **THEN** o gasto da janela volta a zero e um novo instante de reset é calculado a partir da duração configurada

#### Scenario: Chave sem duração configurada

- **WHEN** a chave não define duração de orçamento
- **THEN** o gasto acumula indefinidamente e nenhum reset ocorre

### Requirement: Registro de último uso

O serviço MUST atualizar o instante de último uso da chave a cada requisição autenticada com sucesso. A atualização MUST ser agregada e persistida em lote.

#### Scenario: Atualização após uso

- **WHEN** uma chave é usada em uma requisição autenticada
- **THEN** o instante de último uso passa a refletir essa requisição após a persistência em lote

### Requirement: Autorização administrativa

Os endpoints de gestão de chaves MUST exigir a chave mestra do serviço. Chaves virtuais comuns MUST NOT ser capazes de criar, alterar ou remover chaves.

#### Scenario: Chave comum tentando administrar

- **WHEN** uma chave virtual comum chama `POST /key/generate`
- **THEN** o serviço responde com erro de autorização e nenhuma chave é criada
