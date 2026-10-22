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
internal/analyzer                          проверка репозитория; каркас остального Analyzer
migrations/auth                            идентичности и сессии
migrations/analyzer                        очередь и снимки
migrations/report                          отчёты и история
```

Из `project/backend`: `go run ./cmd/api`. Проверка процесса:
`curl http://127.0.0.1:8080/health` → `200 {"status":"ok"}`.
Остановка — Ctrl+C. Auth/Report/Analyzer пока инициализируют логгер и ждут завершения.
Каркас Analyzer запускается через `go run ./cmd/analyzer`.
Реализованы проверка и нормализация URL, подтверждение публичности через GitHub
REST API. `POST /api/v1/repositories/validate` сохраняет требование сессии из
OpenAPI: account cookie или guest cookie → Auth.ResolveContext → обязательный
VerifiedContext во внутренней ValidateRepositoryCommand. Клиент Auth пока не
подключён: после проверки JSON запрос без session cookie возвращает
401 SESSION_REQUIRED, с cookie — 501 NOT_IMPLEMENTED. Наличие cookie само по
себе не даёт доступа; без проверенного контекста GitHub не вызывается.
Остальные бизнес-маршруты пока возвращают 501; health работает самостоятельно.

Проверка не создаёт анализ. При запуске анализа потребуется независимая повторная
проверка. Frontend подключается отдельной задачей #18468; текущий
`repositoryService` ещё не интегрирован.

```sh
curl -i http://127.0.0.1:8080/api/v1/repositories/validate \
  -H 'Content-Type: application/json' \
  --data '{"repository_url":" https://GitHub.com/Golang/Go.git/ "}'
```

Этот пример пока возвращает 401 SESSION_REQUIRED. После подключения Auth
успех — 200 ValidatedRepository с каноническим lowercase URL, постоянным ID и
метаданными GitHub. commit_sha=null до создания анализа; moved_from=null, кроме
подтверждённого переноса. Коды: 400 URL_REQUIRED/URL_TOO_LONG/
INVALID_REPOSITORY_URL/UNSUPPORTED_PLATFORM; 404 REPOSITORY_NOT_FOUND_OR_PRIVATE;
503 REPOSITORY_CHECK_FAILED/UPSTREAM_RATE_LIMITED; 504 UPSTREAM_TIMEOUT.
Неверный JSON/тип/неизвестное или повторяющееся поле и тело больше 1 MiB —
400 VALIDATION_FAILED согласно существующей таблице ошибок.
Логика проверки и HTTP-ответы проверяются в тестах с подставным Auth-resolver;
полная интеграция с Auth пока отсутствует.
Примеры: [запрос](../../contracts/examples/validate-request.json),
[ответ](../../contracts/examples/validate-response.json),
[перенос](../../contracts/examples/validate-moved.json),
[сбой источника](../../contracts/examples/validate-error.json);
[порядок проверок, переносы и бюджеты](../../docs/architecture/data-flow.md#проверка-url-и-отправка-анализа).

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
| `GITHUB_CHECK_TIMEOUT` | `25s` | Предварительная проверка Analyzer в API; положительный, не более 25s и меньше HTTP_WRITE_TIMEOUT |
| `GITHUB_TOKEN` | пусто | Необязательный сервисный токен GitHub; не пользовательские реквизиты |

Неверные и явно пустые значения обязательных настроек приводят к ошибке запуска.
Пустой GITHUB_TOKEN разрешён: публичные метаданные запрашиваются анонимно.
Каждая попытка ограничена 15s и 5 MiB; максимум один повтор в оставшемся бюджете.
Токен не логируется и не передаётся браузеру. Без него действует анонимный лимит
[GitHub REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
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
