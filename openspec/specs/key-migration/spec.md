# key-migration Specification

## Purpose

Transfere as chaves virtuais existentes do banco do LiteLLM para o schema próprio do serviço, de modo que as chaves já distribuídas aos projetos consumidores continuem funcionando após a troca do proxy.

## Requirements

### Requirement: Importação das chaves existentes

A migração MUST ler as chaves da tabela de tokens do LiteLLM e criar as chaves correspondentes no schema do serviço, preservando o hash do token como identificador. A migração MUST NOT exigir a chave em texto plano, que não é recuperável do banco de origem.

#### Scenario: Chave migrada continua válida

- **WHEN** uma chave existente é migrada e em seguida usada contra o novo serviço
- **THEN** a autenticação é bem-sucedida sem que o cliente tenha trocado de chave

#### Scenario: Chave já removida na origem

- **WHEN** a tabela de origem contém uma chave marcada como removida
- **THEN** essa chave não é criada no destino

### Requirement: Campos preservados na migração

A migração MUST preservar, para cada chave, o gasto acumulado, o limite de orçamento, a duração da janela de orçamento, o instante do próximo reset, o instante de último uso, a data de expiração, o estado de bloqueio, a lista de modelos permitidos, o apelido e as datas de criação e atualização.

#### Scenario: Continuidade do gasto acumulado

- **WHEN** uma chave com gasto acumulado é migrada
- **THEN** a chave no destino inicia com o mesmo gasto acumulado

#### Scenario: Continuidade da janela de orçamento

- **WHEN** uma chave com janela de orçamento em andamento é migrada
- **THEN** o instante de reset no destino é o mesmo da origem, sem antecipar nem adiar o reset

### Requirement: Campos fora do escopo

A migração MUST NOT transferir atributos ligados a funcionalidades fora do escopo do serviço, entre eles vínculos com times, organizações, usuários, permissões de objeto, configuração de rotação automática e ajustes de roteamento.

#### Scenario: Chave vinculada a time

- **WHEN** uma chave de origem possui vínculo com um time
- **THEN** a chave é migrada sem esse vínculo e sem qualquer atributo derivado dele

### Requirement: Execução repetível e verificável

A migração MUST ser idempotente, produzindo o mesmo resultado quando executada novamente sobre o mesmo par de bancos, e MUST emitir um relatório com a quantidade de chaves lidas, criadas, atualizadas e ignoradas, incluindo o motivo de cada exclusão.

#### Scenario: Segunda execução

- **WHEN** a migração é executada duas vezes seguidas sobre os mesmos bancos
- **THEN** a segunda execução não cria chaves duplicadas e não altera os valores já migrados

#### Scenario: Relatório de execução

- **WHEN** a migração termina
- **THEN** é emitido um relatório com as contagens por resultado e o motivo das chaves ignoradas

### Requirement: Modo de verificação sem escrita

A migração MUST oferecer um modo que apenas relata o que faria, sem escrever no banco de destino.

#### Scenario: Simulação

- **WHEN** a migração é executada em modo de verificação
- **THEN** o relatório é produzido e o banco de destino permanece inalterado

### Requirement: Leitura opcional das credenciais de provider

A migração MUST ser capaz de extrair as credenciais de provider armazenadas cifradas na tabela de modelos do LiteLLM, decifrando os dois formatos usados pelo projeto: o formato legado sem marcador e o formato com marcador de versão. Essa extração MUST ser opcional, e a ausência da chave de cifra MUST NOT impedir a migração das chaves virtuais.

#### Scenario: Extração habilitada

- **WHEN** a migração é executada com a chave de cifra do LiteLLM disponível e a extração habilitada
- **THEN** as credenciais de provider são decifradas e apresentadas para configuração do novo serviço

#### Scenario: Extração desabilitada

- **WHEN** a migração é executada sem a chave de cifra
- **THEN** as chaves virtuais são migradas normalmente e a migração informa que as credenciais de provider precisam ser configuradas manualmente
