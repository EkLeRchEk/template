.PHONY: generate migrate migrate-down migrate-status run test

# Автоматически загружаем переменные из .env, если файл существует
-include .env
export

generate:
	go tool oapi-codegen \
		-config internal/config/oapi-codegen.yaml \
		contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-status:
	go tool goose -dir migrations postgres "$(DATABASE_URL)" status

run:
	go run ./cmd/trip-service

test:
	CGO_ENABLED=1 go test -race ./...