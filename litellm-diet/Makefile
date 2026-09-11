.PHONY: up down shell logs fmt fmt-check lint test test-integration cover build tidy vuln catalog

up:
	docker compose up -d

down:
	docker compose down -v

shell:
	docker compose exec app sh

logs:
	docker compose logs -f app

fmt:
	gofmt -l -w .

fmt-check:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "files outside canonical format:"; \
		echo "$$files"; \
		exit 1; \
	fi

lint:
	go vet ./...

test:
	CGO_ENABLED=1 go test -race -cover ./...

test-integration:
	CGO_ENABLED=1 go test -race -p 1 -tags=integration ./...

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

tidy:
	go mod tidy

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

PRICE_SOURCE ?= /tmp/litellm_prices.json

catalog:
	go run ./cmd/gencatalog -source $(PRICE_SOURCE) -output internal/catalog/data/catalog.json
