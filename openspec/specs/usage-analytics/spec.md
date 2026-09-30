# usage-analytics Specification

## Purpose

Registra o consumo diário do gateway por chave e modelo e o expõe em um endpoint compatível com o `GET /user/daily/activity` do LiteLLM, permitindo consultar as métricas de uso dos últimos 7 ou 30 dias como no dashboard do LiteLLM.

## Requirements

### Requirement: Registro diário de uso

O serviço MUST acumular, por dia (UTC), chave e modelo, a quantidade de tokens de entrada e saída, o custo, o total de requisições e a quantidade de requisições bem-sucedidas e com falha. O registro MUST ser agregado em memória e persistido em lote junto com o gasto das chaves, sem uma escrita no banco por requisição. O histórico MUST ser mantido mesmo após a remoção da chave.

#### Scenario: Requisição atendida com sucesso

- **WHEN** uma requisição é atendida com sucesso pelo provider
- **THEN** os tokens e o custo reportados são somados ao registro do dia, da chave e do modelo, e o contador de requisições bem-sucedidas é incrementado

#### Scenario: Falha do provider

- **WHEN** o provider responde com erro, fica inacessível ou retorna uma resposta ilegível
- **THEN** o contador de requisições com falha do dia, da chave e do modelo é incrementado

#### Scenario: Requisição recusada antes do provider

- **WHEN** uma requisição é recusada por autenticação, autorização ou corpo inválido
- **THEN** nenhum registro de uso é gerado

### Requirement: Consulta de atividade diária

O endpoint `GET /user/daily/activity` MUST aceitar `start_date` e `end_date` no formato `YYYY-MM-DD` (UTC), e os filtros opcionais `api_key` (hash da chave) e `model`. Sem datas, o período MUST ser os últimos 7 dias, incluindo o dia corrente. A resposta MUST conter `results`, com um item por dia com uso contendo `date`, `metrics` e `breakdown` por `models`, `providers` e `api_keys`, e `metadata` com os totais do período.

#### Scenario: Últimos 7 dias

- **WHEN** `GET /user/daily/activity` é chamado sem parâmetros de data
- **THEN** a resposta cobre os 7 dias terminando no dia corrente

#### Scenario: Últimos 30 dias

- **WHEN** `start_date` é 29 dias antes de `end_date`
- **THEN** a resposta contém os dias com uso nesse intervalo e os totais do período em `metadata`

#### Scenario: Alias da chave

- **WHEN** o breakdown por `api_keys` inclui uma chave existente
- **THEN** o item contém `metadata.key_alias` com o alias da chave

#### Scenario: Intervalo inválido

- **WHEN** uma data está em formato inválido, `start_date` é posterior a `end_date` ou o intervalo excede 366 dias
- **THEN** o serviço responde `400`

### Requirement: Acesso restrito à master key

O endpoint de atividade diária MUST exigir a master key.

#### Scenario: Chave virtual

- **WHEN** o endpoint é chamado com uma chave virtual ou sem credencial
- **THEN** o serviço responde `401`
