# Root delegating Makefile for ai-mock-interview-v2.
# Targets here proxy into backend/Makefile or frontend npm scripts.
# Run `make help` to list available targets.

.DEFAULT_GOAL := help
.PHONY: help install dev-backend dev-frontend test lint migrate-up migrate-down migrate-create

help: ## Show this help.
	@echo "ai-mock-interview-v2 — root targets"
	@echo
	@echo "Usage: make <target>"
	@echo
	@echo "Targets:"
	@echo "  install         Install backend Go modules and frontend npm packages"
	@echo "  dev-backend     Run Go API with hot reload (air) on :8080"
	@echo "  dev-frontend    Run Vite dev server on :5173"
	@echo "  test            Run backend Go tests + frontend Vitest (single run)"
	@echo "  lint            Run golangci-lint and ESLint"
	@echo "  migrate-up      Apply all pending DB migrations"
	@echo "  migrate-down    Roll back the latest DB migration"
	@echo "  migrate-create  Create a new migration pair (usage: make migrate-create name=add_xxx)"
	@echo "  help            Show this help"

install:       ## Install dependencies for both apps.
	cd frontend && npm install && cd ../backend && go mod tidy

dev-backend:   ## Run backend with air hot-reload.
	$(MAKE) -C backend dev

dev-frontend:  ## Run frontend dev server.
	cd frontend && npm run dev

test:          ## Run all tests across the monorepo.
	$(MAKE) -C backend test && cd frontend && npm test -- --run

lint:          ## Lint backend and frontend.
	$(MAKE) -C backend lint && cd frontend && npm run lint

migrate-up:    ## Apply all pending migrations.
	$(MAKE) -C backend migrate-up

migrate-down:  ## Roll back the most recent migration.
	$(MAKE) -C backend migrate-down

migrate-create: ## Create a new migration pair. Usage: make migrate-create name=add_users_table
	$(MAKE) -C backend migrate-create name=$(name)
