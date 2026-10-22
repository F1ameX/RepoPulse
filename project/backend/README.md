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
internal/auth                             gRPC-каркас Auth Service
internal/report                           каркас Report
internal/pkg/grpcerr                      формат gRPC-ошибок
pkg/proto                                 код, сгенерированный из contracts
internal/analyzer                          каркас Analyzer
migrations/auth                            идентичности и сессии
migrations/analyzer                        очередь и снимки
migrations/report                          отчёты и история
```

Из `project/backend`: `go run ./cmd/api`. Проверка процесса:
`curl http://127.0.0.1:8080/health` → `200 {"status":"ok"}`.
Остановка — Ctrl+C. Report/Analyzer пока инициализируют логгер и ждут завершения.
Auth запускает gRPC-сервер, см. раздел «Auth Service».
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
| `SHUTDOWN_TIMEOUT` | `10s` | API, Auth |
| `AUTH_GRPC_ADDR` | `127.0.0.1:9090` | Auth; host:port gRPC-сервера |
| `AUTH_DATABASE_URL` | — (обязательна) | Auth; строка подключения PostgreSQL, не логируется |

Неверные и явно пустые значения приводят к ошибке запуска.
В PowerShell: `$env:HTTP_ADDR = "127.0.0.1:9090"; go run ./cmd/api`.
В Bash: `HTTP_ADDR=127.0.0.1:9090 go run ./cmd/api`.

Логи пишутся в stdout в JSON. Логгер передаётся через контекст:
`logger.With(ctx, log)` и `logger.From(ctx)`. Если его нет в контексте,
`From` возвращает `slog.Default()`.

## Auth Service

gRPC-сервис по контракту [auth.proto](../../contracts/auth/v1/auth.proto):
AuthService (Register, Login, CreateGuestSession, ResolveContext, Logout) и
стандартный health-check `grpc.health.v1.Health`. Сейчас работают
CreateGuestSession и гостевая часть ResolveContext; Register, Login и Logout
отвечают `UNIMPLEMENTED` с кодом `NOT_IMPLEMENTED`.

### Локальный запуск

Нужны PostgreSQL, применённая миграция схемы `auth` и переменная
`AUTH_DATABASE_URL`; без неё сервис не стартует. Команды — из `project/backend`.

1. PostgreSQL. Пример с Docker (данные удаляются при остановке контейнера):

   ```sh
   docker run -d --rm --name repopulse-postgres -e POSTGRES_PASSWORD=postgres \
     -e POSTGRES_DB=repopulse -p 127.0.0.1:5432:5432 postgres:17-alpine
   export AUTH_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/repopulse?sslmode=disable'
   ```

2. Миграция auth применяется вручную через [Goose](https://github.com/pressly/goose),
   при старте сервиса она не запускается (см. [README миграций](migrations/README.md)):

   ```sh
   go install github.com/pressly/goose/v3/cmd/goose@latest
   goose -dir migrations/auth postgres "$AUTH_DATABASE_URL" up
   ```

3. Запуск сервиса:

   ```sh
   go run ./cmd/auth
   AUTH_GRPC_ADDR=127.0.0.1:9191 go run ./cmd/auth   # другой адрес
   ```

При старте сервис подключается к БД и проверяет её через ping; при ошибке
процесс завершается с кодом 1. В логе появится `gRPC server listening`.
Остановка — Ctrl+C: health переходит в `NOT_SERVING`, сервер завершает
активные вызовы не дольше `SHUTDOWN_TIMEOUT`, затем закрывается пул соединений.
Сервер работает без TLS; внешняя достижимость и защита соединений задаются
конфигурацией запуска.

### Проверка

Health-check — через
[grpc-health-probe](https://github.com/grpc-ecosystem/grpc-health-probe):

```sh
go install github.com/grpc-ecosystem/grpc-health-probe@latest
grpc-health-probe -addr=127.0.0.1:9090                                  # status: SERVING
grpc-health-probe -addr=127.0.0.1:9090 -service=repopulse.auth.v1.AuthService
```

Методы — через [grpcurl](https://github.com/fullstorydev/grpcurl)
(`go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest`). Reflection
не включён, поэтому контракт передаётся явно; `-import-path` указывает на
корневую папку `contracts`.

```sh
# Новая гостевая сессия: token (значение cookie repopulse_guest) и SessionInfo
grpcurl -plaintext -import-path ../../contracts -proto auth/v1/auth.proto \
  -H 'x-request-id: local-1' -d '{}' \
  127.0.0.1:9090 repopulse.auth.v1.AuthService/CreateGuestSession

# Проверка гостя: guestSessionId (id principal) и guestExpiresAt
grpcurl -plaintext -import-path ../../contracts -proto auth/v1/auth.proto \
  -d '{"guest_token": "<token из ответа выше>"}' \
  127.0.0.1:9090 repopulse.auth.v1.AuthService/ResolveContext
```

Для неизвестного, истёкшего или отозванного токена ResolveContext возвращает
`{}` — гостевые поля отсутствуют. grpcurl печатает поля в camelCase.
Созданные записи видны в таблицах `auth.principals` и `auth.sessions`;
в БД хранится только SHA-256 хэш токена.

Ошибки возвращаются по [docs/errors.md](../../docs/errors.md): gRPC status и
`common.v1.ApiError` в trailing metadata `repopulse-error-bin`; `trace_id`
берётся из входящей metadata `x-request-id`. Пакет `internal/pkg/grpcerr`
содержит interceptor и ошибки некорректного запроса (`INVALID_ARGUMENT` /
`VALIDATION_FAILED` с первой ошибкой каждого поля в `field_errors`) и
недоступного пользователя (`UNAUTHENTICATED` / `INVALID_CREDENTIALS`,
«Неверный логин или пароль»). Прочие ошибки скрываются как `INTERNAL_ERROR`.

Порты хранилищ (`UserRepository`, `TokenRepository`, `GuestRepository`) объявлены
в `internal/auth/application/auth/ports.go`, реализации на PostgreSQL —
в `internal/auth/adapter/out/repository`. Валидаторы логина и пароля FR-27 —
`internal/auth/domain/credentials.go`.

### Генерация кода из proto

Сгенерированный код `pkg/proto` хранится в репозитории. После изменения
`contracts/common/v1/common.proto` или `contracts/auth/v1/auth.proto`
выполните из `project/backend` (нужен установленный `protoc`):

```sh
go install tool          # protoc-gen-go и protoc-gen-go-grpc из go.mod
go generate ./pkg/proto
```

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
