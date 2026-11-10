# Trip Service

HTTP-сервис для управления поездками на Go. Проект разработан в рамках лабораторных работ №1 и №2.

Во второй лабораторной сервис переработан с использованием Clean Architecture, добавлены тесты и система наблюдаемости на базе OpenTelemetry.

## Технологии

- Go
- Chi — HTTP-маршрутизация
- PostgreSQL и pgxpool — хранение данных
- Squirrel — построение SQL-запросов
- Goose — миграции базы данных
- OpenAPI и oapi-codegen — HTTP-контракты
- OpenTelemetry — сбор трассировок, метрик и логов
- Jaeger — распределённые трассировки
- Prometheus — метрики
- Grafana — визуализация
- Loki — централизованные логи
- golangci-lint, gofumpt, gci — качество кода

## Возможности

- Создание поездки.
- Получение поездки по UUID.
- Завершение активной поездки.
- Сохранение истории изменения статусов.
- Запрет нескольких активных поездок для одного водителя.
- Транзакционная обработка операций с PostgreSQL.
- Проверки `/health` и `/ready`.
- Graceful shutdown HTTP-сервера.
- Структурированное JSON-логирование.
- Сбор метрик HTTP-запросов.
- Трассировка HTTP-запросов, бизнес-операций и запросов к PostgreSQL.

## Архитектура

Проект разделён на следующие слои:

| Директория | Назначение |
|---|---|
| `cmd/trip-service` | Точка входа в приложение |
| `internal/domain` | Доменные модели и бизнес-правила |
| `internal/usecase` | Сценарии использования |
| `internal/repository` | Реализация доступа к данным |
| `internal/http` | HTTP-обработчики и middleware |
| `internal/infra/postgres` | Подключение к PostgreSQL и транзакции |
| `internal/infra/telemetry` | OpenTelemetry, метрики и логирование |
| `internal/container` | Сборка зависимостей |
| `internal/config` | Конфигурация |
| `internal/model` | Модели хранения данных |
| `internal/generated` | Сгенерированный код OpenAPI |

Основная бизнес-логика находится в слоях `domain` и `usecase`. Работа с PostgreSQL и HTTP вынесена в отдельные компоненты.

## Запуск

Для работы необходимы Go, Docker, tripgoctl и PostgreSQL.

### 1. Запуск окружения

```bash
tripgoctl environment start
tripgoctl connect
```

### 2. Настройка переменных окружения

Создать `.env` на основе `.env.example`.

Указать актуальный `DATABASE_URL` из вывода `tripgoctl connect`.

При использовании OpenTelemetry настроить:

```env
OTEL_SERVICE_NAME=trip-service
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:22318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_RESOURCE_ATTRIBUTES=service.namespace=tripgo,deployment.environment=local
OTEL_TRACES_SAMPLER=parentbased_always_on
```

### 3. Применение миграций

```bash
make migrate
```

### 4. Запуск сервиса

```bash
make run
```

По умолчанию сервер доступен по адресу `http://localhost:8080`.

## HTTP API

| Метод | Маршрут | Описание |
|---|---|---|
| POST | `/api/v1/trips` | Создать поездку |
| GET | `/api/v1/trips/{tripId}` | Получить поездку |
| POST | `/api/v1/trips/{tripId}/finish` | Завершить поездку |
| GET | `/health` | Проверить работу сервиса |
| GET | `/ready` | Проверить готовность сервиса |

## Наблюдаемость

Приложение использует три сигнала OpenTelemetry.

**Трассировки:** Jaeger показывает цепочку выполнения HTTP-запросов, Usecase, транзакций и операций репозитория.

**Метрики:** Prometheus собирает количество запросов и время их обработки. В Grafana создан дашборд `Trip Service Monitoring` с показателями Request/s, HTTP Error Rate и HTTP Latency p95.

**Логи:** структурированные записи `slog` выводятся в терминал и экспортируются в Loki через OpenTelemetry Collector. Записи содержат Trace ID для связи с трассировками.

Адреса инструментов можно получить командой:

```bash
tripgoctl connect
```

## Тестирование

### Unit-тесты

Не требуют подключения к PostgreSQL.

```bash
make test
```

### Интеграционные тесты

Требуют запущенную PostgreSQL и применённые миграции.

```bash
make test-integration
```

Интеграционные тесты помечены директивой:

```go
//go:build integration
```

Проверяются жизненный цикл поездки, конкурентное завершение поездок, откат транзакций и повторное использование транзакции.

### Проверка качества кода

```bash
make check
```

Команда выполняет проверки форматирования через `gofumpt` и `gci`, запускает `golangci-lint`, `go vet`, unit-тесты с Race Detector и `git diff --check`.

Автоматическое форматирование:

```bash
make fmt
```

### Покрытие тестами

```bash
go test -cover ./...
```

Для измерения покрытия с интеграционными тестами:

```bash
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Генерация API

```bash
make generate
```

Команда запускает `oapi-codegen` для генерации кода на основании OpenAPI-контракта.