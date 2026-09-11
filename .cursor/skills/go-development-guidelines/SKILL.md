---
name: go-development-guidelines
description: Padrões de engenharia Go do time, cobrindo estrutura de projeto, tratamento de erros, concorrência, testes, banco de dados, logs, profiling e otimização de memória. Use ao escrever, revisar ou refatorar código Go, ao criar ou alterar arquivos .go, go.mod, Dockerfile ou Makefile de um serviço Go, ao decidir estrutura de pacotes, ao escrever testes ou benchmarks em Go, e ao investigar consumo de memória, alocação ou performance em Go.
---

# Guideline de Desenvolvimento Go

Alvo: Go 1.27. O documento completo está em [Go-development-guidelines.md](Go-development-guidelines.md), com 26 seções.

## Como usar

1. Antes de escrever código Go, leia as seções do guideline relevantes ao que vai construir.
2. Ao escrever, siga as regras inegociáveis abaixo sem precisar reabrir o documento.
3. Antes de encerrar, rode a verificação da última seção.

Leia o documento completo quando a tarefa envolver decisão estrutural: layout de pacotes, desenho de interface, modelo de concorrência, estratégia de teste, acesso a banco ou otimização.

## Regras inegociáveis

**Erros**

- Nunca descarte erro com `_`. Envolva com contexto usando `fmt.Errorf("operação: %w", err)`.
- Mensagem em minúscula, sem pontuação final, sem a palavra "erro".
- Registre o erro uma única vez, na fronteira que decide a resposta. Camadas intermediárias apenas propagam.
- `errors.Is` para erro sentinela, `errors.As` para tipo de erro. `panic` só para bug irrecuperável.

**Assinaturas**

- `ctx context.Context` é sempre o primeiro parâmetro; `error` é sempre o último retorno.
- Nunca guarde `context.Context` em struct.
- Retorne o valor zero junto com o erro, nunca um valor parcialmente preenchido.
- Aceite interfaces, devolva tipos concretos. Defina a interface no pacote que a consome.

**Concorrência**

- Quem cria a goroutine garante que ela termina.
- Toda operação bloqueante recebe `ctx` com prazo.
- `go test -race` é obrigatório antes de qualquer commit.

**Testes**

- Teste de tabela por padrão, com subteste nomeado por caso.
- Prefira fake a mock: fake testa comportamento, mock testa a forma da implementação.
- O nome do teste descreve o comportamento verificado, não o método chamado.

**Memória e performance**

- Dimensione slices e mapas na criação: `make([]T, 0, n)`.
- Faça streaming em vez de bufferizar. `io.Copy` com buffer de `sync.Pool` mantém memória constante por requisição.
- `GOMEMLIMIT` definido um pouco abaixo do limite do container.
- Meça com `pprof` e compare com `benchstat` antes de afirmar ganho.

**Banco de dados**

- Sempre as variantes com contexto: `QueryContext`, `ExecContext`, `QueryRowContext`.
- Parâmetros posicionais sempre. Concatenar valor em SQL é injeção.
- `defer linhas.Close()` e `linhas.Err()` depois do laço.

**Segurança**

- Segredo nunca em código, log ou imagem.
- `crypto/rand` para chaves e identificadores, nunca `math/rand`.
- `subtle.ConstantTimeCompare` para comparar segredo.
- `http.Server` sempre com `ReadHeaderTimeout`, `ReadTimeout` e `WriteTimeout`.

**Estilo**

- `gofmt` decide formatação; não há discussão de estilo.
- Nome do pacote é parte do símbolo: `http.Server`, nunca `http.HTTPServer`.
- Sem prefixo `Get` em getters. Siglas em caixa uniforme: `chaveID`, `parseURL`.
- Não crie pacote `utils` nem diretório `pkg/`.
- Comentário explica o porquê. Todo símbolo exportado tem doc comment começando pelo próprio nome.

## Verificação antes de commit

```bash
gofmt -l . && goimports -w .
go vet ./...
staticcheck ./... && golangci-lint run
go test -race -cover ./...
govulncheck ./...
```

Nenhum apontamento pendente, testes passando e cobertura de 70% ou mais no código crítico.

## Onde procurar no documento completo

| Assunto | Seção |
|---|---|
| Estrutura de diretórios e módulo | 2, 3 |
| Docker e Makefile de desenvolvimento | 4 |
| Nomenclatura e tipos | 5, 6 |
| Funções e tratamento de erros | 7, 8 |
| Concorrência e interfaces | 9, 10 |
| Testes, mocks, integração e carga | 11, 12, 13, 14 |
| Profiling e benchmarks | 15, 16 |
| Otimização e memória | 17 |
| Segurança e padrões de código | 18, 19 |
| Dependências e documentação | 20, 21 |
| Banco de dados | 22 |
| Logs e observabilidade | 23 |
| Regras de ouro e checklist | 24, 25 |
