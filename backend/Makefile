SHELL := /bin/bash

# Load .env so DATABASE_URL_DIRECT is available to migrate without wrapping.
ifneq (,$(wildcard .env))
include .env
export
endif

MIGRATE_DSN ?= $(DATABASE_URL_DIRECT)

.PHONY: dev build run test test-cover test-short lint vet tidy gen \
        migrate-up migrate-down migrate-create migrate-force migrate-version

dev:
	air

build:
	go build -o bin/api ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -race -count=1

test-short:
	go test ./... -count=1 -short

test-cover:
	go test ./... -race -coverprofile=coverage.out

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

gen:
	sqlc generate

migrate-up:
	migrate -path migrations -database "$(MIGRATE_DSN)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_DSN)" down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=add_xxx" && exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

migrate-force:
	@test -n "$(version)" || (echo "usage: make migrate-force version=N" && exit 1)
	migrate -path migrations -database "$(MIGRATE_DSN)" force $(version)

migrate-version:
	migrate -path migrations -database "$(MIGRATE_DSN)" version
