-include .env
export

.PHONY: help run test db-ping db-shell db-status migrate-install migrate-status migrate-up migrate-down migrate-create

help:
	@echo "Available commands:"
	@echo "  make run             - Run the API application"
	@echo "  make test            - Run all unit and integration tests"
	@echo "  make db-ping         - Test PostgreSQL connectivity via psql"
	@echo "  make db-shell        - Open an interactive psql session"
	@echo "  make db-status       - Check local PostgreSQL systemd service status"
	@echo "  make migrate-install - Install goose migration tool"
	@echo "  make migrate-status  - View database migration status"
	@echo "  make migrate-up      - Apply all pending database migrations"
	@echo "  make migrate-down    - Roll back the latest migration"
	@echo "  make migrate-create NAME=<name> - Create a new migration file"

run:
	go run ./cmd/api

test:
	go test -v ./...

db-ping:
	@if [ -z "$$CFT_DB_URL" ]; then echo "CFT_DB_URL is not set"; exit 1; fi
	psql "$$CFT_DB_URL" -c 'SELECT 1;'

db-shell:
	@if [ -z "$$CFT_DB_URL" ]; then echo "CFT_DB_URL is not set"; exit 1; fi
	psql "$$CFT_DB_URL"

db-status:
	systemctl status postgresql

MIGRATION_DIR ?= migrations

migrate-install:
	go install github.com/pressly/goose/v3/cmd/goose@latest

migrate-status:
	@if [ -z "$$CFT_DB_URL" ]; then echo "CFT_DB_URL is not set"; exit 1; fi
	goose -dir $(MIGRATION_DIR) postgres "$$CFT_DB_URL" status

migrate-up:
	@if [ -z "$$CFT_DB_URL" ]; then echo "CFT_DB_URL is not set"; exit 1; fi
	goose -dir $(MIGRATION_DIR) postgres "$$CFT_DB_URL" up

migrate-down:
	@if [ -z "$$CFT_DB_URL" ]; then echo "CFT_DB_URL is not set"; exit 1; fi
	goose -dir $(MIGRATION_DIR) postgres "$$CFT_DB_URL" down

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "NAME is required (e.g. make migrate-create NAME=initial_schema)"; exit 1; fi
	mkdir -p $(MIGRATION_DIR)
	goose -dir $(MIGRATION_DIR) create $(NAME) sql
