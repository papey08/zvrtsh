# USERS_DB_PASSWORD from .env is used by docker compose, migrate and the local backend.
-include .env
export

COMPOSE := docker compose
DB_URL  := postgres://root:$(USERS_DB_PASSWORD)@postgres-users:5432/postgres?sslmode=disable
MIGRATE := $(COMPOSE) run --rm migrate -path=/migrations -database "$(DB_URL)"

# a separate database in the same container for the integration tests
TEST_DB           := zvrtsh_test
TEST_DB_URL       := postgres://root:$(USERS_DB_PASSWORD)@postgres-users:5432/$(TEST_DB)?sslmode=disable
TEST_DATABASE_URL := postgres://root:$(USERS_DB_PASSWORD)@localhost:5432/$(TEST_DB)?sslmode=disable

.PHONY: help env generate generate-back generate-front test test-back test-unit test-front test-db \
	run-all run-back run-front dev-back stop migrate-up migrate-down migrate-create

help: ## show this help
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

env:
	@test -f .env || (echo "create .env from .env.dist first: cp .env.dist .env"; exit 1)

# ── code generation from api/openapi.yaml ──

generate: generate-back generate-front ## generate backend and frontend code from api/openapi.yaml

generate-back: ## generate Go DTOs and server interface
	cd backend && go tool oapi-codegen -config oapi-codegen.yaml ../api/openapi.yaml

generate-front: ## generate TypeScript types of the API
	cd frontend && npm install --no-fund --no-audit && npm run generate

# ── tests ──

test: test-back test-front ## run all checks

test-back: test-db ## run all backend tests, including the ones on Postgres
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race ./...

test-unit: ## run backend tests without the database (the Postgres ones are skipped)
	cd backend && go test -race ./...

test-db: env ## recreate the test database; migrations are applied, reverted and applied again
	$(COMPOSE) up -d --wait postgres-users
	$(COMPOSE) exec -T postgres-users dropdb -U root --if-exists --force $(TEST_DB)
	$(COMPOSE) exec -T postgres-users createdb -U root $(TEST_DB)
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(TEST_DB_URL)" up
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(TEST_DB_URL)" down -all
	$(COMPOSE) run --rm migrate -path=/migrations -database "$(TEST_DB_URL)" up

test-front: ## typecheck and build the frontend
	cd frontend && npm install --no-fund --no-audit && npm run build

# ── running ──

run-all: env ## start everything in docker: frontend http://localhost:3000, API http://localhost:8080
	$(COMPOSE) up --build zvrtsh-front

run-back: env ## start the database, migrations and backend in docker (http://localhost:8080)
	$(COMPOSE) up --build zvrtsh-app

run-front: ## start the frontend dev server with hot reload (http://localhost:5173), needs the backend on :8080
	cd frontend && npm install --no-fund --no-audit && npm run dev

dev-back: env migrate-up ## run the backend locally with go run, the database stays in docker
	cd backend && USERS_DB_HOST=localhost HTTP_SERVER_HOST=localhost go run ./cmd/server

stop: ## stop all containers (the database data is kept)
	$(COMPOSE) down

# ── migrations ──

migrate-up: env ## apply all migrations (starts the database if needed)
	$(COMPOSE) up -d --wait postgres-users
	$(MIGRATE) up

migrate-down: env ## revert the last migration
	$(COMPOSE) up -d --wait postgres-users
	$(MIGRATE) down 1

migrate-create: ## create a new migration: make migrate-create name=add_something
	@test -n "$(name)" || (echo "usage: make migrate-create name=<migration_name>"; exit 1)
	$(COMPOSE) run --rm migrate create -ext sql -dir /migrations -seq $(name)
