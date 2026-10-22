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
internal/analyzer                          модели, сценарии и PostgreSQL-хранилище
migrations/auth                            идентичности и сессии
migrations/analyzer                        очередь и снимки
migrations/report                          отчёты и история
```

Из `project/backend`: `go run ./cmd/api`. Проверка процесса:
`curl http://127.0.0.1:8080/health` → `200 {"status":"ok"}`.
Остановка — Ctrl+C. Report пока инициализирует логгер и ждёт завершения.
Analyzer открывает PostgreSQL-хранилище; см. раздел «Хранение анализа».
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
| `ANALYZER_DATABASE_URL` | обязательно | Analyzer; DSN общей PostgreSQL |
| `ANALYZER_DATABASE_TIMEOUT` | `5s` | Analyzer; срок открытия подключения |

Неверные и явно пустые значения приводят к ошибке запуска.
В PowerShell: `$env:HTTP_ADDR = "127.0.0.1:9090"; go run ./cmd/api`.
В Bash: `HTTP_ADDR=127.0.0.1:9090 go run ./cmd/api`.

Логи пишутся в stdout в JSON. Логгер передаётся через контекст:
`logger.With(ctx, log)` и `logger.From(ctx)`. Если его нет в контексте,
`From` возвращает `slog.Default()`.

## Auth Service

gRPC-сервис по контракту [auth.proto](../../contracts/auth/v1/auth.proto):
AuthService (Register, Login, CreateGuestSession, ResolveContext, Logout) и
стандартный health-check `grpc.health.v1.Health`. Это технический каркас:
все методы AuthService пока отвечают `UNIMPLEMENTED` с кодом `NOT_IMPLEMENTED`;
регистрация, вход и сессии реализуются в следующих задачах.

Запуск из `project/backend`:

```sh
go run ./cmd/auth
AUTH_GRPC_ADDR=127.0.0.1:9191 go run ./cmd/auth   # другой адрес
```

В логе появится `gRPC server listening`. Остановка — Ctrl+C: health переходит
в `NOT_SERVING`, сервер завершает активные вызовы не дольше `SHUTDOWN_TIMEOUT`.
Сервер работает без TLS; внешняя достижимость и защита соединений задаются
конфигурацией запуска.

Health-check проверяется через
[grpc-health-probe](https://github.com/grpc-ecosystem/grpc-health-probe):

```sh
go install github.com/grpc-ecosystem/grpc-health-probe@latest
grpc-health-probe -addr=127.0.0.1:9090                                  # status: SERVING
grpc-health-probe -addr=127.0.0.1:9090 -service=repopulse.auth.v1.AuthService
```

Методы AuthService можно вызвать через [grpcurl](https://github.com/fullstorydev/grpcurl).
Reflection не включён, поэтому контракт передаётся явно (из `project/backend`):

```sh
grpcurl -plaintext -v -import-path ../../contracts -proto auth/v1/auth.proto \
  -H 'x-request-id: local-1' -d '{}' \
  127.0.0.1:9090 repopulse.auth.v1.AuthService/CreateGuestSession
```

Ответ: `Code: Unimplemented` и trailer `repopulse-error-bin` с ApiError.

Ошибки возвращаются по [docs/errors.md](../../docs/errors.md): gRPC status и
`common.v1.ApiError` в trailing metadata `repopulse-error-bin`; `trace_id`
берётся из входящей metadata `x-request-id`. Пакет `internal/pkg/grpcerr`
содержит interceptor и ошибки некорректного запроса (`INVALID_ARGUMENT` /
`VALIDATION_FAILED` с первой ошибкой каждого поля в `field_errors`) и
недоступного пользователя (`UNAUTHENTICATED` / `INVALID_CREDENTIALS`,
«Неверный логин или пароль»). Прочие ошибки скрываются как `INTERNAL_ERROR`.

Интерфейсы хранилищ пользователей (`UserRepository`) и токенов сессий
(`TokenRepository`) объявлены в `internal/auth/application/auth/ports.go`;
реализаций пока нет.

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

## Хранение анализа

Внутренние сценарии `internal/analyzer/application/storage` связаны с PostgreSQL
адаптером через `app.OpenStorage`. Для `go run ./cmd/analyzer` задайте
`ANALYZER_DATABASE_URL` и заранее примените [миграции](migrations/README.md).
Процесс проверяет соединение и ожидает завершения. HTTP, исполнение очереди,
сбор реальных входов и история профиля подключаются отдельными задачами.

- `CreateAccepted` сохраняет уже проверенный репозиторий, серверные UUID,
  Owner из доверенного Auth VerifiedContext, момент оценки, окно 140 дней и версии.
  Повтор пары Owner/request_id возвращает прежний анализ; другой URL отклоняется.
  Это операция хранения принятого задания, а не полный HTTP CreateAnalysis:
  резервация отправки до GitHub и обработка переноса URL относятся к его оркестрации.
- `Start` переводит queued в running и фиксирует срок исполнения.
- `Complete` принимает только типизированные входы и вычисленный результат.
  Metadata снимка/отчёта формируется из сохранённого анализа. Под блокировкой строки
  одна транзакция записывает snapshot, зависимости, уязвимости, report и completed.
  Повторное завершение отклоняется; ошибка записи откатывает все изменения.
- `Get` и `GetResult` проверяют Owner и срок контекста. Исходный guest сохраняет
  доступ после входа при действующем гостевом контексте; отчёт не переносится
  владельцу аккаунта. Чтение не вызывает внешние источники.

Снимок соответствует `contracts/analyzer/v1/schemas.json`: все 12 метрик,
полнота/причины, nullable значения, версии профилей, npm/Maven/Go с origins.
Значения секретов и исходные строки не являются полями модели. Для secret_match
перед записью строится описание только из разрешённого типа находки; текстовый
raw_value SEC_SECRETS отклоняется. Коллекторы и расчёт должны передавать безопасные
нормализованные факты, а не произвольный текст файлов/ошибок.

Интеграционные тесты используют `REPOPULSE_TEST_DATABASE_URL` подключения к
тестовой PostgreSQL с правом CREATE DATABASE. Для каждого прогона создаётся
отдельная БД `repopulse_storage_<случайный UUID>`, применяются foundation Up,
проверяются round trip, доступ, гонки и rollback, затем Down/Up и удаление этой БД.
Без переменной этот тест пропускается; CI поднимает PostgreSQL 17 и запускает его
вместе с `go test -race`. Не указывайте производственную БД.

Пример PowerShell:

```powershell
$env:REPOPULSE_TEST_DATABASE_URL = 'postgres://test_user:test_password@127.0.0.1:5432/postgres?sslmode=disable'
go test -race ./...
```
