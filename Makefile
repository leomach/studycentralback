# Carrega .env automaticamente, se existir: cada KEY=VALUE vira variável de
# make, e `export` propaga todas elas para o ambiente de cada comando de
# receita (inclusive `go run`). Sem isso, `make run` não enxergaria
# DATABASE_URL a menos que você desse `source .env` você mesmo antes.
ifneq (,$(wildcard .env))
    include .env
    export
endif

DATABASE_URL ?= postgres://studycentral:studycentral@localhost:5435/studycentral?sslmode=disable

.PHONY: run test fmt vet db-up db-down migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

db-up:
	docker compose up -d db

db-down:
	docker compose down

# Requer golang-migrate:
#   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1
