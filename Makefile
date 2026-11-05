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

.PHONY: test-integration

# Unit-тесты и интеграционные тесты с PostgreSQL
test-integration:
	CGO_ENABLED=1 go test -race -count=1 -tags=integration ./...

.PHONY: fmt fmt-check lint vet check

# Автоматическое форматирование Go-кода
fmt:
	gofumpt -w ./cmd ./internal
	gci write -s standard -s default -s "prefix(github.com/EkLeRchEk/template)" ./cmd ./internal

# Проверка форматирования без изменения файлов
fmt-check:
	@test -z "$$(gofumpt -l ./cmd ./internal)" || (echo "gofumpt: formatting required"; exit 1)
	@test -z "$$(gci list -s standard -s default -s 'prefix(github.com/EkLeRchEk/template)' ./cmd ./internal)" || (echo "gci: imports formatting required"; exit 1)

# Статический анализ
lint:
	golangci-lint run ./...

vet:
	go vet ./...

# Полная проверка проекта
check: fmt-check lint vet test
	git diff --check