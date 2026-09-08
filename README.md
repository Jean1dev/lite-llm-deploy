# LiteLLM na Railway

Este repositório publica o LiteLLM como um serviço Docker na Railway. A Railway
constrói o `Dockerfile`, fornece a porta do serviço pela variável `PORT` e
executa o entrypoint de produção do LiteLLM.

## Configuração na Railway

Conecte o serviço a este repositório e selecione a branch `main`. Mantenha os
segredos somente nas variables do serviço Railway. O LiteLLM espera, no mínimo:

- `LITELLM_MASTER_KEY`: chave usada para autenticar no proxy.
- `LITELLM_SALT_KEY`: salt permanente usado para criptografar dados no banco.
- `DATABASE_URL`: URL de conexão com o PostgreSQL.
- `REDIS_URL`: URL completa do Redis, incluindo TLS e credenciais quando
  necessário, por exemplo `rediss://default:senha@host:6379`.

Não use uma URL completa em `REDIS_HOST`. Se optar por variáveis separadas,
configure `REDIS_HOST`, `REDIS_PORT`, `REDIS_USERNAME`, `REDIS_PASSWORD` e
`REDIS_SSL=True` individualmente.

O cache de respostas é habilitado em `config.yaml`, com TTL padrão de 600
segundos. O LiteLLM lê `REDIS_URL` diretamente do ambiente; ela não deve ser
repetida em `cache_params`.

O proxy descarta parâmetros OpenAI que o provider de destino não aceita
(`litellm_settings.drop_params`). Sem isso, clientes OpenAI-compatíveis
(LangChain `ChatOpenAI`, SDK OpenAI) falham em modelos Anthropic ao enviar
`presence_penalty` e `frequency_penalty`.

## Desenvolvimento local

O `docker-compose.yml` inicia LiteLLM, PostgreSQL e Redis localmente:

```sh
docker compose up -d
```
