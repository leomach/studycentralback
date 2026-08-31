DATABASE_URL ?= postgres://studycentral:studycentral@localhost:5432/studycentral?sslmode=disable

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
