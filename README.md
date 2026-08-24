# LiteLLM no Heroku

O deploy usa o stack `container` do Heroku. A cada push na branch `main`, o
GitHub Actions envia o commit para o Heroku Git; o Heroku então constrói o
`Dockerfile` definido em `heroku.yml` e inicia o processo `web`.

## Configuração inicial

1. Configure uma única vez o stack do app:

   ```sh
   heroku stack:set container --app NOME_DO_APP
   ```

2. No environment `production` do repositório no GitHub, cadastre:

   - Secret `HEROKU_API_KEY`: chave da conta com acesso ao app.
   - Variable `HEROKU_APP_NAME`: nome do app no Heroku.

3. Mantenha as configurações do LiteLLM somente nas Config Vars do Heroku. No
   mínimo, o serviço espera `LITELLM_MASTER_KEY`, `LITELLM_SALT_KEY`,
   `DATABASE_URL` e `REDIS_URL`.

Depois disso, qualquer push em `main` dispara o deploy. Também é possível
executar o workflow manualmente pela aba Actions.

## Desenvolvimento local

O `docker-compose.yml` inicia LiteLLM, PostgreSQL e Redis localmente:

```sh
docker compose up -d
```
