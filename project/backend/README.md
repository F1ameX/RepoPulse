# RepoPulse backend

[Архитектура №916](../../docs/architecture/repopulse_architecture.md),
[контракты по сервисам](../../contracts/README.md),
[миграции по сервисам](migrations/README.md).

Go 1.26. API Gateway обслуживает HTTP. Auth отвечает за сессии, Report — за
чтение отчётов/историю. Analyzer отвечает за сбор Git/GitHub/OSV,
анализаторы, Score и объяснение. Его типизированный интерфейс отделён от HTTP.

```text
cmd/api, cmd/auth, cmd/report, cmd/analyzer  точки входа
internal/api                              HTTP Gateway
internal/auth, internal/report             каркасы сервисов
internal/analyzer                          каркас Analyzer
migrations/auth                            идентичности и сессии
migrations/analyzer                        очередь и снимки
migrations/report                          отчёты и история
```

Из `project/backend`: `go run ./cmd/api`. Проверка процесса:
`curl http://127.0.0.1:8080/health` → `200 {"status":"ok"}`.
Остановка — Ctrl+C. Auth/Report/Analyzer пока инициализируют логгер и ждут завершения.
Каркас Analyzer запускается через `go run ./cmd/analyzer`.
Бизнес-обработчики подключаются в следующих задачах. HTTP-маршруты из OpenAPI
пока возвращают 501 NOT_IMPLEMENTED; health работает самостоятельно.

## Переменные окружения

Приложение читает окружение процесса. `.env.example` содержит примеры;
файлы `.env` автоматически не загружаются.

| Переменная | По умолчанию | Область |
| --- | --- | --- |
| `LOG_LEVEL` | `info` | Все сервисы; `debug/info/warn/error` |
| `HTTP_ADDR` | `127.0.0.1:8080` | API |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | API |
| `HTTP_READ_TIMEOUT` | `15s` | API |
| `HTTP_WRITE_TIMEOUT` | `30s` | API |
| `HTTP_IDLE_TIMEOUT` | `60s` | API |
| `SHUTDOWN_TIMEOUT` | `10s` | API |

Неверные и явно пустые значения приводят к ошибке запуска.
В PowerShell: `$env:HTTP_ADDR = "127.0.0.1:9090"; go run ./cmd/api`.
В Bash: `HTTP_ADDR=127.0.0.1:9090 go run ./cmd/api`.

Логи пишутся в stdout в JSON. Логгер передаётся через контекст:
`logger.With(ctx, log)` и `logger.From(ctx)`. Если его нет в контексте,
`From` возвращает `slog.Default()`.

## Контракты

[OpenAPI Gateway](../../contracts/gateway/v1/openapi.json) задаёт HTTP.
JSON Schema и `.proto` разделены по сервисам в `contracts/`.
[Карта полей](../../docs/field-map.md) описывает обе формы данных,
[flow](../../docs/architecture/data-flow.md) — их передачу и сохранение.
Код классов контрактов каждый сервис получает своими инструментами.

Каркас Gateway содержит локальную структуру ErrorEnvelope для существующих
ответов ошибок: trace_id (X-Request-ID) и field_errors. request_id отправки анализа
остаётся отдельным ключом.

## Проверки

Из `project/backend`: `go test -race ./...`, `go vet ./...`, `go build ./...`.
[Результат технической проверки Git](../../docs/git-check.md).
