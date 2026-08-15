SHELL := /usr/bin/env bash

SERVER_DIR := apps/server
WEB_DIR    := apps/web
COMPOSE    := docker compose -f docker-compose.dev.yml
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X main.version=$(VERSION)

# The dev stack publishes Postgres on loopback, so host-run tooling connects to
# localhost rather than the in-network "postgres" hostname used in production.
DEV_DATABASE_URL := postgres://shorturl:shorturl@localhost:5432/shorturl?sslmode=disable

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Start Postgres, Redis, the Go server, and the Nuxt dashboard
	@./scripts/dev.sh

.PHONY: up
up: ## Start the development infrastructure containers
	@$(COMPOSE) up -d --wait

.PHONY: down
down: ## Stop the development infrastructure containers
	@$(COMPOSE) down

.PHONY: logs
logs: ## Follow container logs
	@$(COMPOSE) logs -f

.PHONY: test
test: ## Run all tests
	@cd $(SERVER_DIR) && go test ./...

.PHONY: load-test
load-test: ## Continuously create links; requires SHORTURL_API_KEY
	@./scripts/load-create-links.sh

.PHONY: lint
lint: ## Vet the Go code and typecheck the dashboard
	@cd $(SERVER_DIR) && gofmt -l . | tee /dev/stderr | (! read -r)
	@cd $(SERVER_DIR) && go vet ./...
	@cd $(WEB_DIR) && bun run typecheck

.PHONY: fmt
fmt: ## Format the Go code
	@cd $(SERVER_DIR) && gofmt -w .

.PHONY: build
build: build-server build-web ## Build the server binary and the dashboard

.PHONY: build-server
build-server: ## Build the Go binary into apps/server/bin/shorturl
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/shorturl ./cmd/server

.PHONY: build-web
build-web: ## Build the Nuxt dashboard
	@cd $(WEB_DIR) && bun run build

.PHONY: migrate
migrate: ## Apply pending database migrations to the dev database
	@cd $(SERVER_DIR) && DATABASE_URL="$(DEV_DATABASE_URL)" go run ./cmd/server migrate

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration on the dev database
	@cd $(SERVER_DIR) && DATABASE_URL="$(DEV_DATABASE_URL)" go run ./cmd/server migrate-down

.PHONY: sqlc
sqlc: ## Regenerate the type-safe query layer from db/queries
	@command -v sqlc >/dev/null 2>&1 || { \
		echo "sqlc not found. Install it with:"; \
		echo "  go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"; \
		exit 1; }
	@cd $(SERVER_DIR)/db && sqlc generate
	@echo "regenerated apps/server/internal/store"

.PHONY: deps
deps: ## Install dashboard dependencies and download Go modules
	@cd $(SERVER_DIR) && go mod download
	@cd $(WEB_DIR) && bun install

##@ Production

.PHONY: install
install: ## Generate secrets and check prerequisites for a deployment
	@./scripts/install.sh

.PHONY: prod-build
prod-build: ## Build the production images
	@VERSION=$(VERSION) docker compose build

.PHONY: prod-up
prod-up: ## Start the production stack
	@VERSION=$(VERSION) docker compose up -d

.PHONY: prod-down
prod-down: ## Stop the production stack (volumes are kept)
	@docker compose down

.PHONY: prod-logs
prod-logs: ## Follow production logs
	@docker compose logs -f

.PHONY: prod-ps
prod-ps: ## Show production service status
	@docker compose ps

.PHONY: backup
backup: ## Archive .env, the database, and the TLS certificates
	@./scripts/backup.sh

.PHONY: restore
restore: ## Restore a backup: make restore ARCHIVE=backups/shorturl-....tar.gz
	@test -n "$(ARCHIVE)" || { echo "usage: make restore ARCHIVE=<path>"; exit 1; }
	@./scripts/restore.sh "$(ARCHIVE)"

.PHONY: clean
clean: ## Remove build output
	@rm -rf $(SERVER_DIR)/bin $(WEB_DIR)/.output $(WEB_DIR)/.nuxt
