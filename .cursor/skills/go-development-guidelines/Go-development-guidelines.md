# Guideline de Desenvolvimento Go

Referência de engenharia para projetos Go. Alvo: Go 1.27.

## Project Stack

Bibliotecas declaradas para este projeto, listadas para referência.

**Especificadas**

- **Driver de banco**: pgx v5.10.0 — driver e pool nativos para PostgreSQL, sem `database/sql` obrigatório — https://github.com/jackc/pgx
- **HTTP**: `net/http` (stdlib) — servidor e cliente HTTP, sem framework — https://pkg.go.dev/net/http

**Preenchidas por padrão da linguagem**

- **Testes**: `testing` (stdlib) + testify v1.11.1 para asserções — https://github.com/stretchr/testify
- **Logging**: `log/slog` (stdlib) — logging estruturado com níveis e atributos — https://pkg.go.dev/log/slog
- **Formatação**: gofmt, goimports — https://pkg.go.dev/cmd/gofmt
- **Linting**: `go vet`, staticcheck, golangci-lint v2.13.2 — https://golangci-lint.run
- **Build**: `go build`, make — https://pkg.go.dev/cmd/go

> Todos os exemplos de código deste documento usam apenas biblioteca padrão ou recursos nativos da linguagem. Os princípios valem independentemente das bibliotecas escolhidas.

---

## 1. Princípios Fundamentais

### 1.1 Filosofia e Estilo

Go tem uma única formatação canônica e um conjunto pequeno de convenções aceitas por toda a comunidade. Não há espaço para estilo pessoal.

- `gofmt` decide formatação. Discussão sobre chaves, indentação ou alinhamento não existe.
- Erros são valores retornados, não exceções. O fluxo de erro é explícito no código.
- Concorrência se comunica por canais ou por memória protegida, nunca por acidente.
- A biblioteca padrão vem primeiro. Dependência é dívida.
- `go vet` e staticcheck rodam antes de qualquer commit.

### 1.2 Clareza acima de brevidade

- Nomes comunicam intenção no contexto do pacote. `buf` dentro de um leitor é claro; `b` num escopo de 40 linhas não é.
- O comprimento do nome cresce com o tamanho do escopo. Variável de laço pode ser `i`; campo exportado de struct não.
- Código explícito supera abstração esperta. Interfaces genéricas prematuras são o principal vetor de complexidade acidental em Go.
- Otimize depois de medir. `pprof` e `testing.B` existem justamente para que a decisão não seja por intuição.

---

## 2. Inicialização do Projeto

### 2.1 Criando um novo projeto

```bash
mkdir meu-servico && cd meu-servico
go mod init github.com/org/meu-servico
go mod edit -go=1.27
mkdir -p cmd/servidor internal
```

O caminho do módulo é a URL de importação. Use o endereço real do repositório, mesmo em projeto privado, para que a importação funcione sem `replace`.

### 2.2 Gerenciamento de dependências

```bash
go get github.com/jackc/pgx/v5@latest
go get -u ./...
go get github.com/jackc/pgx/v5@none
go mod tidy
go mod verify
go mod download
```

- `go get` adiciona ou atualiza; `@none` remove.
- `go mod tidy` sincroniza `go.mod` e `go.sum` com os imports reais. No Go 1.27 ele consolida múltiplos blocos `require` em dois blocos, diretos e indiretos.
- `go mod verify` confere que os módulos em cache correspondem aos hashes de `go.sum`.
- Versione `go.mod` e `go.sum`. Nunca os edite à mão, exceto por `go mod edit`.

### 2.3 Fixando a versão do toolchain

```bash
go mod edit -toolchain=go1.27.1
go version
```

A diretiva `toolchain` faz o comando `go` baixar e usar exatamente aquela versão, o que remove diferenças entre a máquina do desenvolvedor e a CI.

---

## 3. Estrutura do Projeto

```
meu-servico/
├── cmd/
│   └── servidor/
│       └── main.go          binário; apenas wiring e configuração
├── internal/
│   ├── config/              carregamento e validação de configuração
│   ├── http/                handlers, middlewares, roteamento
│   ├── domain/              tipos e regras de negócio, sem I/O
│   └── storage/             acesso a banco e adaptadores externos
├── migrations/              arquivos SQL versionados
├── testdata/                fixtures lidas por testes
├── Dockerfile
├── docker-compose.yaml
├── .dockerignore
├── Makefile
├── go.mod
└── go.sum
```

Regras que sustentam esse layout:

- `internal/` é imposto pelo compilador: nada fora do módulo consegue importar. É a fronteira de API pública real do projeto.
- Um binário por diretório em `cmd/`. `main` faz composição, não lógica.
- Não crie `pkg/`. Ou o código é interno, e vai em `internal/`, ou é API pública, e vai na raiz.
- Testes ficam ao lado do código, com sufixo `_test.go`; fixtures em `testdata/`, ignorado pelas ferramentas.
- Evite pacote `utils` ou `common`. Nomeie pelo que o pacote faz, não pelo que ele é.

---

## 4. Desenvolvimento em Container (Docker)

### 4.1 Filosofia

Todo projeto usa Docker no desenvolvimento: ambiente idêntico entre desenvolvedores, nenhum runtime instalado na máquina, mesma versão em desenvolvimento e produção, dependências isoladas.

### 4.2 Arquivos necessários

`Dockerfile`, `docker-compose.yaml` e `.dockerignore` na raiz do módulo.

### 4.3 Dockerfile de desenvolvimento

Imagem oficial menor disponível, variante Alpine, fixada na versão estável atual. Sem multi-stage em desenvolvimento.

```dockerfile
FROM golang:1.27-alpine

RUN apk add --no-cache git make curl

WORKDIR /app

ENV CGO_ENABLED=0 \
    GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/go-mod

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["sleep", "infinity"]
```

### 4.4 Docker Compose

```yaml
services:
  app:
    build: .
    volumes:
      - .:/app
      - go-cache:/cache
    environment:
      DATABASE_URL: postgres://app:app@db:5432/app?sslmode=disable
      GOMEMLIMIT: 256MiB
    ports:
      - "8080:8080"
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:18-alpine
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
      interval: 5s
      retries: 10

volumes:
  go-cache:
  pgdata:
```

### 4.5 .dockerignore

```
.git
.gitignore
bin/
dist/
coverage.out
*.test
*.prof
README.md
```

### 4.6 Comandos essenciais

| Operação | Comando |
|---|---|
| Subir ambiente | `docker compose up -d` |
| Ver logs | `docker compose logs -f app` |
| Executar aplicação | `docker compose exec app go run ./cmd/servidor` |
| Rodar testes | `docker compose exec app go test ./...` |
| Shell interativo | `docker compose exec app sh` |
| Reconstruir imagem | `docker compose build --no-cache app` |
| Derrubar ambiente | `docker compose down -v` |

### 4.7 Makefile

```makefile
.PHONY: up down test lint fmt build

up:
	docker compose up -d

down:
	docker compose down -v

fmt:
	go fmt ./... && goimports -w .

lint:
	go vet ./... && staticcheck ./... && golangci-lint run

test:
	go test -race -cover ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/servidor ./cmd/servidor
```

### 4.8 Boas práticas

- Mantenha `CGO_ENABLED=0` para produzir binário estático e permitir imagem final `scratch` ou `distroless`.
- Monte `GOCACHE` e `GOMODCACHE` em volume nomeado: sem isso cada build recompila a stdlib.
- Em produção use multi-stage e copie apenas o binário, com `GOMEMLIMIT` alinhado ao limite do container. Em desenvolvimento, não use multi-stage.

---

## 5. Convenções de Nomenclatura

| Elemento | Convenção | Exemplo |
|---|---|---|
| Pacote | minúsculo, uma palavra, sem underscore | `storage`, `httpapi` |
| Arquivo | minúsculo com underscore | `chave_virtual.go` |
| Tipo | CamelCase | `ChaveVirtual` |
| Função exportada | CamelCase | `NovaChave` |
| Função interna | camelCase | `calcularCusto` |
| Constante | CamelCase, sem SCREAMING_CASE | `LimitePadrao` |
| Interface | sufixo `-er` quando descreve ação | `Reader`, `Autorizador` |
| Variável de erro | prefixo `Err` | `ErrChaveInvalida` |
| Tipo de erro | sufixo `Error` | `ValidacaoError` |

Pontos que a revisão de código cobra:

- O nome do pacote é parte do nome do símbolo. `http.HTTPServer` é redundante; `http.Server` é correto.
- Getters não levam prefixo `Get`. Use `Nome()`, não `GetNome()`.
- Siglas mantêm caixa uniforme: `URL`, `ID`, `HTTP`. Escreva `chaveID`, nunca `chaveId`.
- Receivers são curtos e consistentes no tipo inteiro: `func (c *Chave)`, sempre `c`.
- Nunca use `self` ou `this` como receiver.

---

## 6. Tipos e Sistema de Tipos

Go tem tipagem estática, forte e nominal, com inferência apenas na declaração curta.

### 6.1 Declaração

```go
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

type Chave struct {
	Hash      string
	Alias     string
	Gasto     float64
	Orcamento *float64
	Expira    *time.Time
}

type Autorizador interface {
	Autorizar(ctx context.Context, hash, modelo string) error
}
```

Go não tem enum. O padrão é um tipo nomeado sobre um tipo base mais um bloco de constantes, o que dá segurança na assinatura das funções sem custo em runtime.

### 6.2 Segurança de tipos

- Crie tipos nomeados para identificadores. `func Buscar(id ChaveID)` impede passar um `UsuarioID` por engano; `func Buscar(id string)` não impede nada.
- Use ponteiro apenas quando ausência é um estado válido e distinto de zero. `Orcamento *float64` diferencia "sem orçamento" de "orçamento zero".
- Evite `any`. Quando precisar, converta na fronteira e valide com type assertion de duas formas.

```go
valor, ok := entrada.(string)
if !ok {
	return fmt.Errorf("esperado string, recebido %T", entrada)
}
```

### 6.3 Alocação e inicialização

```go
chaves := make(map[string]Chave, 512)
modelos := make([]string, 0, 340)

var buf bytes.Buffer
buf.Grow(4096)
```

Dimensione mapas e slices na criação sempre que a ordem de grandeza for conhecida. Um slice que cresce de zero até 340 elementos realoca cerca de dez vezes e copia todo o conteúdo a cada realocação.

O valor zero deve ser útil. `var buf bytes.Buffer` e `var mu sync.Mutex` já estão prontos para uso, e projetar seus tipos assim elimina construtores desnecessários.

---

## 7. Funções e Métodos

### 7.1 Assinaturas

```go
func BuscarChave(ctx context.Context, db *sql.DB, hash string) (Chave, error) {
	const q = `SELECT hash, alias, gasto FROM chaves WHERE hash = $1`

	var c Chave
	err := db.QueryRowContext(ctx, q, hash).Scan(&c.Hash, &c.Alias, &c.Gasto)
	if errors.Is(err, sql.ErrNoRows) {
		return Chave{}, fmt.Errorf("buscar chave %s: %w", hash[:8], ErrChaveNaoEncontrada)
	}
	if err != nil {
		return Chave{}, fmt.Errorf("buscar chave %s: %w", hash[:8], err)
	}
	return c, nil
}
```

Convenções obrigatórias:

- `context.Context` é sempre o primeiro parâmetro, sempre nomeado `ctx`. Nunca guarde context em struct.
- `error` é sempre o último retorno.
- Retorne o valor zero do tipo junto com o erro, não um valor parcialmente preenchido.

### 7.2 Retornos e erros

**Ruim**

```go
func Carregar(caminho string) *Config {
	dados, _ := os.ReadFile(caminho)
	var c Config
	json.Unmarshal(dados, &c)
	return &c
}
```

O erro de leitura e o de parsing somem. O chamador recebe um `*Config` zerado e descobre o problema muito depois, em outro lugar.

**Bom**

```go
func Carregar(caminho string) (Config, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return Config{}, fmt.Errorf("ler config %q: %w", caminho, err)
	}

	var c Config
	if err := json.Unmarshal(dados, &c); err != nil {
		return Config{}, fmt.Errorf("parsear config %q: %w", caminho, err)
	}
	return c, nil
}
```

### 7.3 Boas práticas

- Uma responsabilidade por função. Se o nome precisa de "e", são duas funções.
- Até três ou quatro parâmetros. Acima disso, agrupe num struct de opções.
- Retornos nomeados só em funções curtas ou quando um `defer` precisa alterar o erro. Nunca use `return` nu em função longa.
- Prefira aceitar interfaces e devolver tipos concretos.
- Passe slices e maps cientes de que são referências: mutação dentro da função é visível fora.

---

## 8. Tratamento de Erros

### 8.1 Filosofia

Erro é valor. Não há exceção, não há stack unwinding, não há `catch`. `panic` é para bug de programação irrecuperável, nunca para controle de fluxo.

```go
var (
	ErrChaveNaoEncontrada = errors.New("chave não encontrada")
	ErrOrcamentoExcedido  = errors.New("orçamento excedido")
)

type ValidacaoError struct {
	Campo  string
	Motivo string
}

func (e *ValidacaoError) Error() string {
	return fmt.Sprintf("campo %s inválido: %s", e.Campo, e.Motivo)
}

func Debitar(c *Chave, custo float64) error {
	if custo < 0 {
		return &ValidacaoError{Campo: "custo", Motivo: "valor negativo"}
	}
	if c.Orcamento != nil && c.Gasto+custo > *c.Orcamento {
		return fmt.Errorf("debitar %.6f na chave %s: %w", custo, c.Alias, ErrOrcamentoExcedido)
	}
	c.Gasto += custo
	return nil
}
```

Inspeção no chamador:

```go
var ve *ValidacaoError
switch {
case errors.As(err, &ve):
	responder(w, http.StatusBadRequest, ve.Campo)
case errors.Is(err, ErrOrcamentoExcedido):
	responder(w, http.StatusPaymentRequired, "orçamento excedido")
default:
	responder(w, http.StatusInternalServerError, "erro interno")
}
```

### 8.2 Convenções

**Ruim**

```go
if err != nil {
	log.Println(err)
	return errors.New("falha na operação")
}
```

Registra e propaga ao mesmo tempo, gerando log duplicado em cada camada, e descarta o erro original, o que impede `errors.Is` e `errors.As` de funcionarem acima.

**Bom**

```go
if err != nil {
	return fmt.Errorf("sincronizar chave %s: %w", hash[:8], err)
}
```

Envolve com contexto e preserva a cadeia. O log acontece uma única vez, na fronteira de I/O.

### 8.3 Boas práticas

- Nunca descarte erro com `_`. A única exceção defensável é em `defer` de operação sem efeito observável, e ainda assim documente.
- Mensagens em minúsculas, sem pontuação final e sem a palavra "erro": elas são concatenadas em cadeia.
- Adicione contexto que identifique a operação e os identificadores envolvidos, nunca segredos.
- Use `%w` para envolver e `%v` quando quiser deliberadamente cortar a cadeia.
- Erros sentinela para condições que o chamador testa; tipos de erro quando há dados estruturados a expor.
- Registre o erro apenas na borda que decide a resposta. Camadas intermediárias apenas propagam.

---

## 9. Concorrência e Paralelismo

### 9.1 Modelo

Goroutines são threads gerenciadas pelo runtime, com pilha inicial de poucos kilobytes que cresce sob demanda. Criar milhares é normal; criar sem controlar o encerramento é vazamento.

```go
func processar(ctx context.Context, entradas <-chan Requisicao) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)

	for req := range entradas {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			atender(ctx, req)
		}()
	}
	wg.Wait()
}
```

A partir do Go 1.22 cada iteração de `for` cria uma nova variável de laço, então capturar `req` no closure é seguro. Em código antigo você ainda encontra `req := req` como proteção.

### 9.2 Sincronização

- `chan` para transferir posse de dados e sinalizar eventos.
- `sync.Mutex` e `sync.RWMutex` para proteger estado compartilhado; mantenha a região crítica mínima.
- `sync.WaitGroup` para esperar um conjunto conhecido de goroutines.
- `sync/atomic` e os tipos `atomic.Int64`, `atomic.Pointer[T]` para contadores e trocas sem lock.
- `context.Context` para cancelamento e prazo, propagado por toda a cadeia.

```go
type Contador struct {
	mu     sync.Mutex
	gastos map[string]float64
}

func (c *Contador) Somar(hash string, valor float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gastos[hash] += valor
}

func (c *Contador) Drenar() map[string]float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	saida := c.gastos
	c.gastos = make(map[string]float64, len(saida))
	return saida
}
```

Trocar o mapa em vez de iterar e zerar mantém o lock preso por tempo constante.

### 9.3 Boas práticas

- Quem cria a goroutine é responsável por garantir que ela termina.
- Toda operação bloqueante recebe `ctx` com prazo. Sem prazo, um provider lento vira acúmulo de goroutines.
- Encerramento gracioso: pare de aceitar, drene o que está em voo, persista o pendente.

```go
func servir(ctx context.Context, srv *http.Server) error {
	erros := make(chan error, 1)
	go func() { erros <- srv.ListenAndServe() }()

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
		desliga, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelar()
		return srv.Shutdown(desliga)
	}
}
```

### 9.4 Armadilhas comuns

- Escrever em canal sem leitor e sem buffer bloqueia para sempre.
- Fechar canal do lado do leitor, ou fechar duas vezes, causa panic. Quem escreve fecha.
- `time.After` dentro de `select` em laço aloca um timer por iteração; use `time.NewTimer` e reaproveite.
- Mapa nativo não é seguro para acesso concorrente. Leitura e escrita simultâneas derrubam o processo.
- `defer` dentro de laço só executa no fim da função. Extraia o corpo para uma função.
- Rode sempre `go test -race`. O detector encontra o que revisão não encontra.

---

## 10. Interfaces e Abstrações

### 10.1 Desenho

Interfaces em Go são satisfeitas implicitamente. Isso muda quem as define: o consumidor declara o que precisa, não o produtor o que oferece.

```go
type Repositorio interface {
	Buscar(ctx context.Context, hash string) (Chave, error)
}

type Servico struct {
	repo Repositorio
}
```

Quanto menor a interface, mais implementações servem. `io.Reader` tem um método e é o tipo mais reutilizado da linguagem.

### 10.2 Implementação

```go
type repoPostgres struct {
	db *sql.DB
}

func (r *repoPostgres) Buscar(ctx context.Context, hash string) (Chave, error) {
	return BuscarChave(ctx, r.db, hash)
}

var _ Repositorio = (*repoPostgres)(nil)
```

A declaração de variável em branco garante em tempo de compilação que o tipo satisfaz a interface, sem custo em runtime.

### 10.3 Composição

```go
type Leitor interface {
	Buscar(ctx context.Context, hash string) (Chave, error)
}

type Escritor interface {
	Salvar(ctx context.Context, c Chave) error
}

type Armazenamento interface {
	Leitor
	Escritor
}
```

Regras que a revisão cobra:

- Defina a interface no pacote que a consome, não junto da implementação.
- Aceite interfaces, devolva structs.
- Não crie interface com uma única implementação e sem necessidade de teste. Adicione quando o segundo caso aparecer.
- Cuidado com interface nula contendo tipo concreto nulo: `var p *T = nil` atribuído a uma interface faz `iface != nil` ser verdadeiro.

---

## 11. Testes Unitários

### 11.1 Estrutura

Framework nativo, sem runner externo. Arquivo `_test.go` no mesmo pacote; use o sufixo `_test` no nome do pacote quando quiser testar apenas a API exportada.

```go
package chave

import (
	"errors"
	"testing"
)

func TestDebitarOrcamentoExcedido(t *testing.T) {
	limite := 1.0
	c := &Chave{Alias: "app", Gasto: 0.9, Orcamento: &limite}

	err := Debitar(c, 0.5)

	if !errors.Is(err, ErrOrcamentoExcedido) {
		t.Fatalf("erro = %v, esperado %v", err, ErrOrcamentoExcedido)
	}
	if c.Gasto != 0.9 {
		t.Errorf("gasto = %v, esperado inalterado 0.9", c.Gasto)
	}
}
```

O nome do teste descreve o comportamento verificado, não o método chamado. `t.Fatal` interrompe; `t.Error` acumula falhas.

### 11.2 Testes de tabela

O formato padrão em Go. Um caso por entrada do slice, subteste nomeado por caso.

```go
func TestResolverProvider(t *testing.T) {
	casos := []struct {
		nome     string
		modelo   string
		provider Provider
		querErro bool
	}{
		{"openai", "openai/gpt-4o-mini", ProviderOpenAI, false},
		{"anthropic", "anthropic/claude-haiku-4-5", ProviderAnthropic, false},
		{"sem barra", "gpt-4o-mini", "", true},
		{"prefixo desconhecido", "cohere/command", "", true},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()

			got, err := ResolverProvider(c.modelo)
			if c.querErro {
				if err == nil {
					t.Fatalf("esperava erro para %q", c.modelo)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if got != c.provider {
				t.Errorf("provider = %q, esperado %q", got, c.provider)
			}
		})
	}
}
```

### 11.3 Asserções

A stdlib não tem biblioteca de asserção: compare e reporte com `t.Errorf`, incluindo valor obtido e esperado nessa ordem. Para structs e slices use `reflect.DeepEqual`. Recursos de apoio da stdlib:

- `t.Helper()` marca funções auxiliares para que o erro aponte a linha do teste.
- `t.Cleanup(fn)` registra limpeza que roda mesmo em falha; `t.TempDir()` e `t.Setenv(k, v)` já se limpam sozinhos.
- `testing/synctest` executa código dependente de tempo em relógio virtual, sem sleeps reais.

### 11.4 Comandos

```bash
go test ./...
go test -v ./internal/chave
go test -run TestDebitarOrcamentoExcedido ./internal/chave
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
go tool cover -html=coverage.out
go test -count=1 -short ./...
```

`-count=1` desliga o cache de resultados. `-race` deve ser padrão na CI.

---

## 12. Mocks e Testabilidade

### 12.1 Estratégias

Go não precisa de biblioteca de mock na maioria dos casos. Como interfaces são implícitas e pequenas, um struct de teste de dez linhas resolve.

```go
type repoFake struct {
	chaves map[string]Chave
	erro   error
}

func (r *repoFake) Buscar(_ context.Context, hash string) (Chave, error) {
	if r.erro != nil {
		return Chave{}, r.erro
	}
	c, ok := r.chaves[hash]
	if !ok {
		return Chave{}, ErrChaveNaoEncontrada
	}
	return c, nil
}
```

Geradores como `mockgen` valem quando a interface é grande ou quando a verificação de chamadas importa. Para interfaces de um ou dois métodos, o custo de manutenção do código gerado não se paga.

### 12.2 Injeção de dependência

Injeção por construtor, sem framework e sem container.

```go
type Servico struct {
	repo Repositorio
	log  *slog.Logger
	agora func() time.Time
}

func NovoServico(repo Repositorio, log *slog.Logger) *Servico {
	return &Servico{repo: repo, log: log, agora: time.Now}
}
```

Injetar `agora` como campo torna o tempo controlável no teste sem tocar em relógio global.

### 12.3 Test doubles

- **Stub**: retorna valor fixo.
- **Fake**: implementação simplificada e funcional, como um repositório em mapa.
- **Spy**: registra as chamadas recebidas para inspeção posterior.
- **Mock**: verifica expectativas de chamada e falha se não forem cumpridas.

Prefira fake a mock. Fake testa comportamento; mock testa a forma da implementação e quebra em toda refatoração.

Para HTTP, `httptest.NewServer` cobre cliente e `httptest.NewRecorder` cobre handler. O Go 1.27 adiciona `httptest.NewTestServer`, com rede em memória, adequado para uso com `testing/synctest`.

---

## 13. Testes de Integração

### 13.1 Estrutura e organização

Separe por build tag, para que o conjunto rápido continue rápido.

```go
//go:build integration

package storage_test

import "testing"

func TestRepositorioBuscar(t *testing.T) {
	db := abrirBancoDeTeste(t)
	t.Cleanup(func() { db.Close() })
}
```

A alternativa sem tag é `testing.Short()` combinada com `-short`, útil quando o teste é lento mas não exige infraestrutura externa.

```go
func TestSincronizacaoCompleta(t *testing.T) {
	if testing.Short() {
		t.Skip("pulado em modo curto")
	}
}
```

### 13.2 Execução seletiva

```bash
go test ./...
go test -tags=integration ./...
go test -short ./...
go test -tags=integration -run TestRepositorio ./internal/storage
```

### 13.3 Dependências reais

Teste contra o banco real, não contra um simulacro: diferenças de dialeto SQL só aparecem no banco de verdade. Duas abordagens estabelecidas:

- **testcontainers-go**: sobe o container a partir do próprio teste, com ciclo de vida atrelado ao pacote de teste.
- **docker compose**: o serviço já está no ambiente de desenvolvimento e a CI reaproveita a mesma definição.

Em qualquer uma delas, isole cada teste em transação com rollback ou em schema próprio, para que a ordem de execução não importe.

---

## 14. Testes de Carga e Estresse

### 14.1 Ferramentas

- `go test -bench` com `b.RunParallel` para carga dentro do processo.
- `vegeta` e `k6` para carga externa sobre HTTP, medindo latência por percentil.
- `go test -race` sob carga revela corridas que passam despercebidas em execução sequencial.
- `-cpu=1,4,8` mede o comportamento em diferentes graus de paralelismo.

### 14.2 Benchmarks de carga

```go
func BenchmarkHandlerChatCompletions(b *testing.B) {
	srv := novoServidorDeTeste(b)
	corpo := []byte(`{"model":"openai/gpt-4o-mini","messages":[]}`)

	b.ReportAllocs()
	b.SetBytes(int64(len(corpo)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(corpo))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != 200 {
				b.Fatalf("status = %d", rec.Code)
			}
		}
	})
}
```

### 14.3 Testes de concorrência

```bash
go test -race -count=100 -run TestContadorConcorrente ./internal/chave
go test -bench=. -benchtime=30s -cpu=1,2,4,8 ./...
```

Repetir o teste cem vezes com detector de corrida ligado é a forma prática de expor condições de corrida raras.

---

## 15. Profiling e Diagnóstico

### 15.1 CPU e memória

```bash
go test -bench=BenchmarkRota -cpuprofile=cpu.out -memprofile=mem.out ./internal/http
go tool pprof -http=:8081 cpu.out
go tool pprof -top -nodecount=20 mem.out
go tool pprof -alloc_space mem.out
```

Em serviço em execução, exponha `net/http/pprof` numa porta administrativa, nunca na porta pública.

```go
import _ "net/http/pprof"

go func() {
	log.Println(http.ListenAndServe("127.0.0.1:6060", nil))
}()
```

```bash
go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/heap
go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/profile?seconds=30
curl -o trace.out http://127.0.0.1:6060/debug/pprof/trace?seconds=5
go tool trace trace.out
```

### 15.2 Ferramentas de diagnóstico

| Perfil | Para que serve |
|---|---|
| `profile` | tempo de CPU por função |
| `heap` | memória viva e alocações acumuladas |
| `allocs` | todas as alocações desde o início |
| `goroutine` | pilhas de todas as goroutines |
| `goroutineleak` | goroutines permanentemente bloqueadas, disponível a partir do Go 1.27 |
| `mutex` | contenção em locks |
| `block` | espera em operações bloqueantes |
| `trace` | linha do tempo de escalonamento, GC e rede |

Perfis de `mutex` e `block` exigem ativação explícita com `runtime.SetMutexProfileFraction` e `runtime.SetBlockProfileRate`.

### 15.3 Análise

```bash
GODEBUG=gctrace=1 ./bin/servidor
go tool pprof -base=antes.out depois.out
```

Método que funciona: capture o perfil sob carga representativa, olhe `-top` para achar os poucos pontos que dominam, use `-http` para navegar até a linha, corrija, e compare com `-base` para provar o ganho. Perfil capturado em máquina ociosa mede o aquecimento, não o gargalo.

---

## 16. Benchmarks

### 16.1 Escrevendo benchmarks

```go
func BenchmarkExtrairModelo(b *testing.B) {
	corpo := []byte(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"oi"}]}`)

	b.ReportAllocs()
	b.SetBytes(int64(len(corpo)))

	for b.Loop() {
		if _, err := ExtrairModelo(corpo); err != nil {
			b.Fatal(err)
		}
	}
}
```

`b.Loop()` substitui o laço `for i := 0; i < b.N; i++` e impede que o compilador elimine a chamada por não ter efeito observável. `b.ReportAllocs()` deve estar em todo benchmark: alocação por operação costuma ser a métrica que importa.

### 16.2 Sub-benchmarks

```go
func BenchmarkEncaminhar(b *testing.B) {
	for _, tam := range []int{1 << 10, 1 << 14, 1 << 18} {
		b.Run(fmt.Sprintf("corpo=%dKiB", tam/1024), func(b *testing.B) {
			dados := bytes.Repeat([]byte("x"), tam)
			b.SetBytes(int64(tam))
			b.ReportAllocs()

			for b.Loop() {
				if err := Encaminhar(io.Discard, bytes.NewReader(dados)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
```

Variar o tamanho da entrada é como se demonstra que o consumo de memória é constante e não proporcional.

### 16.3 Execução e comparação

```bash
go test -bench=. -benchmem ./...
go test -bench=BenchmarkEncaminhar -benchtime=5s -count=10 ./internal/http > novo.txt
go install golang.org/x/perf/cmd/benchstat@latest
benchstat antes.txt novo.txt
```

Uma medição isolada é ruído. Rode com `-count=10` e compare com `benchstat`, que aplica teste estatístico e informa a variação com intervalo de confiança.

---

## 17. Otimização

### 17.1 Princípios

Meça antes. O perfil quase sempre aponta um lugar diferente da intuição. Colha as frutas baixas primeiro: alocação em laço quente, conversão desnecessária, buffer não dimensionado. Documente qualquer trade-off que sacrifique clareza, porque a próxima pessoa vai querer "limpar" o código.

Em Go a métrica que mais importa raramente é ciclo de CPU: é alocação por operação. Cada alocação é trabalho agora e trabalho depois, quando o coletor precisar varrê-la.

### 17.2 Otimizações comuns

```go
func juntar(partes []string) string {
	var b strings.Builder
	n := 0
	for _, p := range partes {
		n += len(p)
	}
	b.Grow(n)
	for _, p := range partes {
		b.WriteString(p)
	}
	return b.String()
}
```

Concatenar com `+` em laço cria uma string nova por iteração. `strings.Builder` com `Grow` faz uma alocação.

```go
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

func copiar(dst io.Writer, src io.Reader) (int64, error) {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	return io.CopyBuffer(dst, src, *buf)
}
```

`sync.Pool` compensa em objetos grandes e de vida curta reutilizados sob concorrência. Para objetos pequenos, o alocador do Go é mais rápido que a sincronização do pool. O Go 1.27 reduziu em até 30% o custo de alocação de objetos abaixo de 80 bytes, o que estreita ainda mais a faixa em que o pool vale a pena.

### 17.3 Otimização de memória

- Dimensione slices e mapas na criação com `make(T, 0, n)`.
- Não guarde subslice de um buffer grande: o array inteiro fica vivo. Copie o trecho necessário.
- Passe structs grandes por ponteiro; structs pequenos por valor, que evita escape para o heap.
- Prefira `[]byte` a `string` no caminho de dados e converta apenas na fronteira.
- Faça streaming em vez de bufferizar. `io.Copy` com buffer de pool mantém a memória constante por requisição, independente do tamanho da resposta.
- Para JSON grande, use `encoding/json/jsontext`, introduzido no Go 1.27, que permite ler tokens em fluxo sem materializar a estrutura inteira.

```bash
GOGC=200 GOMEMLIMIT=256MiB ./bin/servidor
```

`GOMEMLIMIT` define um teto flexível de memória que o coletor respeita, aumentando a frequência de coleta ao se aproximar do limite. Em container, defina-o um pouco abaixo do limite do orquestrador: é a diferença entre coletar mais e ser morto por falta de memória. `GOGC` alto reduz coletas ao custo de mais memória residente; os dois juntos dão um envelope previsível.

### 17.4 Performance básica

- `strconv.Itoa` em vez de `fmt.Sprintf("%d", n)`, que passa por reflexão.
- Verifique escape com `go build -gcflags='-m'` antes de supor onde um valor foi alocado.
- Evite `defer` em laço muito quente; fora dele o custo é desprezível.
- Reaproveite `*http.Client` e `*sql.DB`: ambos são seguros para uso concorrente e mantêm pool interno.
- `regexp.MustCompile` no nível do pacote, nunca dentro do handler.

---

## 18. Segurança

### 18.1 Práticas essenciais

- Segredo nunca no código nem em imagem. Leia de variável de ambiente ou de um gerenciador de segredos, e valide na inicialização.
- Valide toda entrada externa na fronteira, com limite de tamanho: `http.MaxBytesReader` no corpo e `srv.ReadHeaderTimeout` no cabeçalho.
- TLS em toda comunicação externa. Não desabilite verificação de certificado, nem em ambiente interno.
- Rate limit por identidade, com `golang.org/x/time/rate` ou equivalente.
- Dependências atualizadas e auditadas em cada build.
- Menor privilégio: usuário não-root no container, sistema de arquivos somente leitura quando possível.

```go
srv := &http.Server{
	Addr:              ":8080",
	Handler:           mux,
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      120 * time.Second,
	IdleTimeout:       90 * time.Second,
	MaxHeaderBytes:    1 << 16,
}
```

Um `http.Server` sem timeouts aceita conexão que nunca termina de enviar cabeçalho, e cada uma custa uma goroutine.

### 18.2 Ferramentas

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
go list -m -u all
go vet ./...
staticcheck ./...
```

`govulncheck` é o scanner oficial e reporta apenas vulnerabilidades em código efetivamente alcançável, o que elimina a maior parte dos falsos positivos.

### 18.3 Segurança nas fronteiras

- Compare segredos com `subtle.ConstantTimeCompare`, nunca com `==`, para não vazar informação por tempo de resposta.
- Gere identificadores e chaves com `crypto/rand`, jamais com `math/rand`.
- Copie slices e maps recebidos antes de guardá-los: o chamador ainda tem a referência e pode mutá-los.
- Nunca coloque segredo, corpo de requisição ou token em log.
- Sanitize o que volta ao cliente: mensagem de erro interna não deve expor caminho de arquivo, SQL nem nome de host interno.

```go
func chaveNova() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gerar chave: %w", err)
	}
	return "sk-" + base64.RawURLEncoding.EncodeToString(b), nil
}
```

---

## 19. Padrões de Código

### 19.1 Early return

**Ruim**

```go
func processar(r *Requisicao) error {
	if r != nil {
		if r.Valida() {
			if r.Autorizada() {
				return executar(r)
			}
			return ErrNaoAutorizada
		}
		return ErrInvalida
	}
	return ErrNula
}
```

**Bom**

```go
func processar(r *Requisicao) error {
	if r == nil {
		return ErrNula
	}
	if !r.Valida() {
		return ErrInvalida
	}
	if !r.Autorizada() {
		return ErrNaoAutorizada
	}
	return executar(r)
}
```

Trate o caso de erro e saia. O caminho feliz fica alinhado à esquerda e legível de cima a baixo.

### 19.2 Separação de responsabilidades

Regra de negócio não conhece HTTP nem SQL. Isso é o que torna o teste rápido e a implementação substituível.

```go
func (s *Servico) Debitar(ctx context.Context, hash string, custo float64) error {
	c, err := s.repo.Buscar(ctx, hash)
	if err != nil {
		return err
	}
	if err := Debitar(&c, custo); err != nil {
		return err
	}
	return s.repo.Salvar(ctx, c)
}
```

`Debitar` sobre a struct é pura, decidida por teste de tabela. O serviço orquestra I/O.

### 19.3 DRY

Extraia depois da terceira ocorrência, não da segunda. Duplicação é mais barata que a abstração errada, e em Go a comunidade aceita explicitamente um pouco de repetição em troca de código óbvio.

### 19.4 Escopo de variáveis

```go
if err := validar(r); err != nil {
	return err
}

for i, item := range itens {
	if item.Vazio() {
		continue
	}
	processar(i, item)
}
```

Declare no menor escopo possível, usando a forma com inicialização em `if` e `switch`. Uma variável declarada no topo da função e usada 40 linhas abaixo obriga quem lê a rastrear tudo entre os dois pontos.

---

## 20. Gerenciamento de Dependências

### 20.1 Princípios

- Biblioteca padrão primeiro. `net/http`, `log/slog`, `encoding/json`, `database/sql` e `testing` cobrem a maior parte de um serviço.
- Antes de adicionar, verifique manutenção ativa, número de dependências transitivas e compatibilidade de licença.
- Minimalismo. Cada dependência é superfície de ataque, tempo de build e trabalho de atualização.
- Versionamento explícito. `go.mod` e `go.sum` versionados, sem `replace` apontando para caminho local em `main`.

### 20.2 Comandos

```bash
go list -m all
go list -m -u all
go get -u ./...
go get -u=patch ./...
go mod tidy
go mod why github.com/alguma/lib
go mod graph | grep alguma/lib
govulncheck ./...
go clean -modcache
```

`go mod why` responde por que uma dependência entrou no grafo, que é a informação que falta quando `go mod tidy` traz algo inesperado.

---

## 21. Comentários e Documentação

### 21.1 Comentários no código

Comentário explica o porquê. O que o código faz já está escrito no código, e um comentário que descreve a linha seguinte envelhece e passa a mentir.

Escreva comentário para registrar uma restrição que o código não consegue mostrar: um limite imposto por um sistema externo, uma decisão contraintuitiva com motivo, uma referência a especificação ou incidente.

### 21.2 Documentação de API

Doc comment fica imediatamente antes da declaração, sem linha em branco, e começa com o nome do símbolo.

```go
// Debitar soma custo ao gasto acumulado da chave.
//
// Retorna ErrOrcamentoExcedido quando o resultado ultrapassaria o orçamento
// configurado, deixando o gasto inalterado. Chaves sem orçamento nunca
// retornam esse erro.
func Debitar(c *Chave, custo float64) error {
```

O formato aceita listas, blocos de código indentados e links entre símbolos com colchetes, como `[Chave]`.

### 21.3 Documentação de pacote

Um comentário de pacote por módulo lógico, no arquivo `doc.go` quando for extenso.

```go
// Package chave implementa chaves virtuais de acesso, incluindo autenticação
// por hash, controle de orçamento com janela de reset e registro de último uso.
//
// O gasto é agregado em memória e persistido em lote; leituras logo após uma
// requisição podem observar um valor com poucos segundos de atraso.
package chave
```

```bash
go doc ./internal/chave
go doc ./internal/chave Debitar
go doc github.com/jackc/pgx/v5@v5.10.0
```

---

## 22. Banco de Dados

### 22.1 Abordagem

Go oferece três caminhos, e a comunidade pende fortemente para os dois primeiros.

| Abordagem | Vantagem | Custo |
|---|---|---|
| SQL puro com `database/sql` | controle total, zero mágica, plano de query previsível | escrever o mapeamento à mão |
| Gerador a partir de SQL | tipos gerados do schema real, erro em tempo de compilação | passo de geração no build |
| ORM | menos código para CRUD simples | consultas opacas, dificuldade em otimizar, reflexão em runtime |

Para serviço com caminho quente sensível a latência ou memória, SQL puro ou gerado é a escolha padrão. ORM em Go raramente se paga.

### 22.2 Conexão e driver

`database/sql` é uma camada genérica com pool de conexões embutido. O driver concreto é registrado por import anônimo.

```go
func abrir(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir banco: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectar ao banco: %w", err)
	}
	return db, nil
}
```

`sql.Open` não conecta: apenas valida o DSN, e o `PingContext` é o que prova que o banco responde. `SetMaxIdleConns` igual a `SetMaxOpenConns` evita que o pool feche e reabra conexões sob carga oscilante; `SetConnMaxLifetime` protege contra conexões mortas por um balanceador no meio do caminho. O `*sql.DB` é um pool, não uma conexão: crie um por processo, compartilhe entre goroutines e feche uma única vez no encerramento.

### 22.3 Consultas com parâmetros

```go
func ListarPorProvider(ctx context.Context, db *sql.DB, provider string, limite int) ([]Chave, error) {
	const q = `SELECT hash, alias, gasto, orcamento FROM chaves
		WHERE provider = $1 AND bloqueada = false
		ORDER BY ultimo_uso DESC LIMIT $2`

	linhas, err := db.QueryContext(ctx, q, provider, limite)
	if err != nil {
		return nil, fmt.Errorf("listar chaves de %q: %w", provider, err)
	}
	defer linhas.Close()

	chaves := make([]Chave, 0, limite)
	for linhas.Next() {
		var c Chave
		if err := linhas.Scan(&c.Hash, &c.Alias, &c.Gasto, &c.Orcamento); err != nil {
			return nil, fmt.Errorf("ler linha: %w", err)
		}
		chaves = append(chaves, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("iterar chaves: %w", err)
	}
	return chaves, nil
}
```

Dois detalhes que causam bug em produção: `defer linhas.Close()` logo após checar o erro, e `linhas.Err()` depois do laço, porque `Next()` retornando falso pode significar erro e não fim.

Escrita em transação:

```go
func Persistir(ctx context.Context, db *sql.DB, gastos map[string]float64) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	const q = `UPDATE chaves SET gasto = gasto + $2, ultimo_uso = now() WHERE hash = $1`
	for hash, valor := range gastos {
		if _, err = tx.ExecContext(ctx, q, hash, valor); err != nil {
			return fmt.Errorf("atualizar gasto de %s: %w", hash[:8], err)
		}
	}
	return tx.Commit()
}
```

**Ruim**

```go
q := "SELECT hash FROM chaves WHERE alias = '" + alias + "'"
linhas, err := db.Query(q)
```

**Bom**

```go
const q = `SELECT hash FROM chaves WHERE alias = $1`
linhas, err := db.QueryContext(ctx, q, alias)
```

Concatenar valor em SQL é injeção. O único trecho montável dinamicamente é estrutura, como coluna de ordenação, e ainda assim a partir de uma lista fechada.

### 22.4 Migrações

Migração é arquivo SQL versionado em par (`0001_criar_chaves.up.sql` e `0001_criar_chaves.down.sql`), aplicado em ordem, registrado numa tabela de controle e nunca editado depois de aplicado. Correção se faz com nova migração.

O ecossistema resolve isso com ferramentas dedicadas, executadas como binário ou embarcadas com `embed.FS`. Aplique migração num passo separado do start: réplicas competindo para migrar o mesmo banco é fonte clássica de incidente.

### 22.5 Boas práticas

- Sempre `QueryContext`, `ExecContext` e `QueryRowContext`, com timeout por consulta.
- Parâmetros posicionais sempre. Concatenação de string é injeção de SQL.
- Índice para toda coluna usada em filtro ou ordenação frequente. Confirme com `EXPLAIN ANALYZE`.
- Transação explícita e curta. Transação aberta durante chamada de rede prende conexão do pool.
- Trate `sql.ErrNoRows` como caso de negócio, com `errors.Is`, não como falha de infraestrutura.

---

## 23. Logs e Observabilidade

### 23.1 Níveis

`log/slog` traz quatro níveis, e a escala é numérica, o que permite níveis intermediários quando necessário.

| Nível | Uso |
|---|---|
| `DEBUG` | detalhe de diagnóstico, desligado em produção |
| `INFO` | evento relevante do fluxo normal |
| `WARN` | condição inesperada da qual o serviço se recuperou |
| `ERROR` | falha que afetou a operação em curso |

Go não tem `FATAL` como nível. O equivalente é registrar em `ERROR` e encerrar com `os.Exit(1)`, sempre no `main`, porque `os.Exit` não executa `defer`.

### 23.2 Logs estruturados

Log estruturado é par chave-valor, não frase. Isso é o que permite filtrar e agregar sem expressão regular.

```go
func novoLogger(nivel string, saida io.Writer) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(nivel)); err != nil {
		l = slog.LevelInfo
	}

	h := slog.NewJSONHandler(saida, &slog.HandlerOptions{
		Level:     l,
		AddSource: l <= slog.LevelDebug,
	})
	return slog.New(h)
}

func main() {
	logger := novoLogger(os.Getenv("LOG_LEVEL"), os.Stdout)
	slog.SetDefault(logger)

	if err := executar(context.Background(), logger); err != nil {
		logger.Error("encerrando por falha", slog.String("erro", err.Error()))
		os.Exit(1)
	}
}
```

Escreva sempre em `stdout`. Quem coleta, rotaciona e roteia é a plataforma, não a aplicação.

### 23.3 Implementação com contexto

```go
func middlewareLog(logger *slog.Logger, prox http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()

		l := logger.With(
			slog.String("req_id", requestID(r)),
			slog.String("metodo", r.Method),
			slog.String("rota", r.URL.Path),
		)
		ctx := context.WithValue(r.Context(), chaveLogger{}, l)

		rec := &gravador{ResponseWriter: w, status: http.StatusOK}
		prox.ServeHTTP(rec, r.WithContext(ctx))

		l.LogAttrs(ctx, slog.LevelInfo, "requisição atendida",
			slog.Int("status", rec.status),
			slog.Duration("duracao", time.Since(inicio)),
		)
	})
}
```

`logger.With` cria um logger derivado com atributos fixos, então o identificador de correlação não precisa ser repetido em cada chamada. `LogAttrs` evita a conversão para `any` dos argumentos variádicos e é a forma indicada em caminho quente.

**Ruim**

```go
logger.Info(fmt.Sprintf("chamada de %s com chave %s", r.RemoteAddr, chave))
```

**Bom**

```go
logger.LogAttrs(ctx, slog.LevelInfo, "chamada recebida",
	slog.String("req_id", reqID),
	slog.String("chave_hash", hash[:8]),
)
```

Frase interpolada não é filtrável nem agregável, e joga o segredo dentro do coletor de logs. Nunca registre chave, token, corpo de requisição ou dado pessoal: registre identificador de correlação e identidade em forma não reversível.

### 23.4 Métricas e observabilidade

Instrumente a fronteira: chamadas HTTP recebidas, chamadas externas emitidas, consultas ao banco. Instrumentar função interna gera volume sem informação.

O conjunto mínimo é latência por percentil, taxa de erro, throughput e saturação de recursos, complementado pelas métricas de runtime que o Go expõe nativamente por `runtime/metrics`: memória residente, goroutines vivas, pausa de coletor e uso do pool de conexões via `db.Stats()`.

```go
mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
	ctx, cancelar := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancelar()

	if err := db.PingContext(ctx); err != nil {
		http.Error(w, "banco indisponível", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
})
```

Liveness responde apenas se o processo está vivo, sem tocar em dependência; readiness responde se ele consegue atender. Confundir os dois faz o orquestrador reiniciar um serviço saudável porque o banco oscilou. Mantenha a cardinalidade dos rótulos sob controle: rótulo com identificador de usuário ou de requisição multiplica séries temporais até derrubar o sistema de métricas.

---

## 24. Regras de Ouro

1. **Simplicidade.** Código óbvio vence código esperto. Se precisa de comentário para ser entendido, reescreva antes de comentar.
2. **Erros explícitos.** Nunca descarte um erro. Envolva com contexto usando `%w`, registre uma vez na fronteira, e deixe a decisão para quem tem informação para decidir.
3. **Testes.** Teste de tabela por padrão, `-race` sempre, fake em vez de mock. Teste comportamento observável, não estrutura interna.
4. **Documentação.** Todo símbolo exportado tem doc comment começando pelo próprio nome. Todo pacote tem um comentário explicando sua razão de existir.
5. **Performance medida.** `pprof` e `benchstat` antes e depois. Otimização sem medição é dívida disfarçada de melhoria.

---

## 25. Checklist Pré-Commit

### Código

- [ ] `gofmt` e `goimports` aplicados
- [ ] `go vet ./...` sem apontamentos
- [ ] `staticcheck ./...` e `golangci-lint run` sem erro crítico
- [ ] `go build ./...` compila sem aviso

### Testes

- [ ] `go test -race ./...` passa
- [ ] Cobertura de 70% ou mais no código crítico
- [ ] Testes de integração executados quando a mudança toca I/O
- [ ] Benchmarks comparados com `benchstat` quando há mudança em caminho quente

### Qualidade

- [ ] Erros tratados explicitamente, com contexto e sem descarte
- [ ] Recursos liberados com `defer`: `Close`, `Rollback`, `cancel`
- [ ] Goroutines com encerramento garantido e operações com prazo
- [ ] Nenhum segredo no código, em log ou em imagem
- [ ] `govulncheck ./...` sem vulnerabilidade alcançável

### Documentação

- [ ] Símbolos exportados documentados
- [ ] README atualizado quando comando ou configuração mudou
- [ ] Comentários explicam o porquê, não o que

### Docker

- [ ] `Dockerfile` constrói sem erro
- [ ] `docker compose up -d` deixa o ambiente pronto
- [ ] Aplicação inicia no container e responde ao endpoint de liveness

---

## 26. Referências

### Documentação oficial

- Effective Go — https://go.dev/doc/effective_go
- Go Code Review Comments — https://go.dev/wiki/CodeReviewComments
- Go Memory Model — https://go.dev/ref/mem
- A Guide to the Go Garbage Collector — https://go.dev/doc/gc-guide
- Managing dependencies — https://go.dev/doc/modules/managing-dependencies
- Go 1.27 release notes — https://go.dev/blog/go1.27

### Guias de estilo

- Google Go Style Guide — https://google.github.io/styleguide/go/
- Uber Go Style Guide — https://github.com/uber-go/guide/blob/master/style.md
- Go Proverbs — https://go-proverbs.github.io

### Ferramentas

- gofmt e goimports — https://pkg.go.dev/cmd/gofmt
- staticcheck — https://staticcheck.dev
- golangci-lint — https://golangci-lint.run
- govulncheck — https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
- benchstat — https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- testcontainers-go — https://golang.testcontainers.org

### Testes, performance e biblioteca padrão

- Pacote testing — https://pkg.go.dev/testing
- Profiling Go Programs — https://go.dev/blog/pprof
- Diagnostics — https://go.dev/doc/diagnostics
- log/slog — https://pkg.go.dev/log/slog
- database/sql — https://pkg.go.dev/database/sql
- net/http — https://pkg.go.dev/net/http
- encoding/json/v2 e jsontext — https://pkg.go.dev/encoding/json/v2

### Comunidade

- Go Blog — https://go.dev/blog
- Go Forum — https://forum.golangbridge.org
- Awesome Go — https://awesome-go.com
