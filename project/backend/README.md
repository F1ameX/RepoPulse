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
Analyzer открывает PostgreSQL-хранилище; см.
[пошаговый запуск](#запуск-analyzer-с-postgresql).
Auth запускает gRPC-сервер, см. раздел «Auth Service».
Каркас Analyzer запускается через `go run ./cmd/analyzer`.
В С1 работают GET /api/v1/auth/session, POST /api/v1/auth/guest-session и
POST /api/v1/repositories/validate через проверку cookie в Auth. В С2 добавлены
POST /api/v1/analyses и GET /api/v1/analyses/{analysis_id} с тем же Auth-клиентом.
Остальные бизнес-маршруты пока возвращают 501 NOT_IMPLEMENTED.
Валидация URL не создаёт анализ; job появляется только после принятия POST /analyses.

Весь проект с PostgreSQL и автоматическим применением миграций запускается
через [Docker Compose](../../README.md#docker--спринт-1). Goose устанавливается
в отдельный контейнер; на хосте он нужен только для запуска вне Compose.

Весь проект с PostgreSQL и автоматическим применением миграций запускается
через [Docker Compose](../../README.md#docker--спринт-1). Goose устанавливается
в отдельный контейнер; на хосте он нужен только для запуска вне Compose.

## Запуск Analyzer с PostgreSQL

Нужны Go 1.26, запущенная PostgreSQL 17 и её утилиты `psql`/`createdb` в PATH.
Ниже команды для PowerShell, начиная из корня репозитория. В примере PostgreSQL
слушает `127.0.0.1:5432`, административная роль — `postgres`, база и роль
приложения — `repopulse`. Если у вас другие параметры, замените их в командах и DSN.

1. Один раз создайте роль и базу для локальной разработки. `psql` и `createdb`
   запросят пароль административной роли, если он требуется сервером.
   Если роль и база уже существуют, используйте их и переходите к следующему шагу.

   ```powershell
   psql -h 127.0.0.1 -p 5432 -U postgres -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE repopulse LOGIN PASSWORD 'local-password';"
   createdb -h 127.0.0.1 -p 5432 -U postgres -O repopulse repopulse
   ```

   `local-password` — пароль роли приложения для этого локального примера.
   Все три схемы `auth`, `analyzer`, `report` создаются миграциями в одной базе.

2. Установите CLI [Goose](https://github.com/pressly/goose), если он ещё не установлен,
   и добавьте каталог установки Go-инструментов в PATH текущего терминала:

   ```powershell
   go install github.com/pressly/goose/v3/cmd/goose@v3.27.0
   $gooseBin = go env GOBIN
   if (-not $gooseBin) { $gooseBin = Join-Path (go env GOPATH) 'bin' }
   $env:Path += ";$gooseBin"
   goose --version
   ```

3. Перейдите в backend и задайте окружение в том же терминале:

   ```powershell
   cd project/backend
   $env:ANALYZER_DATABASE_URL = 'postgres://repopulse:local-password@127.0.0.1:5432/repopulse?sslmode=disable'
   $env:ANALYZER_DATABASE_TIMEOUT = '5s'
   ```

   `.env.example` служит образцом: копирование в `.env` само по себе не задаёт
   окружение процесса. `sslmode=disable` в примере относится к локальной PostgreSQL.

4. Примените миграции в порядке `auth → analyzer → report`, с отдельной таблицей
   версий Goose для каждого каталога:

   ```powershell
   goose -dir migrations/auth -table goose_auth_version postgres "$env:ANALYZER_DATABASE_URL" up
   goose -dir migrations/analyzer -table goose_analyzer_version postgres "$env:ANALYZER_DATABASE_URL" up
   goose -dir migrations/report -table goose_report_version postgres "$env:ANALYZER_DATABASE_URL" up
   ```

   Каждая команда должна завершиться успешно. Применение миграций и их откат
   подробнее описаны в [README миграций](migrations/README.md).

5. Запустите Analyzer из `project/backend`:

   ```powershell
   go run ./cmd/analyzer
   ```

   Успешный запуск подтверждает JSON-лог с `service: analyzer-service` и
   `msg: Analysis storage ready`. Процесс остаётся запущенным и ожидает завершения;
   остановка — `Ctrl+C`, в логе появится `Service stopped`.

На этом этапе процесс Analyzer проверяет БД и открывает хранилище. Он ещё не
открывает HTTP/gRPC-порт и не запускает анализ репозитория. `GET /health`
на порту 8080 относится к отдельно запущенному API Gateway. Проверка операций
чтения/записи описана в разделе [«Хранение анализа»](#хранение-анализа).

Если запуск завершается с `ANALYZER_DATABASE_URL is required`, проверьте окружение
текущего терминала. `analyzer database is unavailable` означает, что нужно проверить
работу PostgreSQL, адрес и реквизиты DSN. При `apply auth/analyzer/report migrations`
примените все три набора миграций к той же базе, которая указана в DSN.

## Переменные окружения

Приложение читает окружение процесса. `.env.example` содержит примеры;
файлы `.env` автоматически не загружаются.

| Переменная | По умолчанию | Область |
| --- | --- | --- |
| `LOG_LEVEL` | `info` | Все сервисы; `debug/info/warn/error` |
| `HTTP_ADDR` | `127.0.0.1:8080` | API |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | API |
| `HTTP_READ_TIMEOUT` | `15s` | API |
| `HTTP_WRITE_TIMEOUT` | `35s` | API; больше AUTH_REQUEST_TIMEOUT + GITHUB_CHECK_TIMEOUT + 3s |
| `HTTP_IDLE_TIMEOUT` | `60s` | API |
| `SHUTDOWN_TIMEOUT` | `10s` | API, Auth |
| `AUTH_GRPC_TARGET` | `127.0.0.1:9090` | API; адрес gRPC Auth, в Compose `auth:9090` |
| `AUTH_REQUEST_TIMEOUT` | `3s` | API; каждый RPC; суммарный бюджет Auth и GitHub должен помещаться в HTTP_WRITE_TIMEOUT |
| `PUBLIC_ORIGIN` | `http://localhost:3000` | API; точный Origin frontend для POST гостя, проверки URL и запуска анализа |
| `COOKIE_SECURE` | `true` | API; в локальном HTTP Compose — false, для HTTPS — true |
| `GITHUB_CHECK_TIMEOUT` | `25s` | API; бюджет GitHub-проверки, меньше HTTP_WRITE_TIMEOUT |
| `GITHUB_TOKEN` | пусто | API; необязательный серверный токен GitHub, не логируется |
| `AUTH_GRPC_ADDR` | `127.0.0.1:9090` | Auth; host:port gRPC-сервера |
| `AUTH_DATABASE_URL` | — (обязательна) | Auth; строка подключения PostgreSQL, не логируется |
| `ANALYZER_DATABASE_URL` | обязательно | API и Analyzer; DSN общей PostgreSQL |
| `ANALYZER_DATABASE_TIMEOUT` | `5s` | API и Analyzer; срок открытия подключения |
| `ANALYZER_VERSIONS` | обязательно | API; JSON с восемью версиями common.Versions, пример в .env.example |

Неверные и явно пустые значения приводят к ошибке запуска.
В PowerShell: `$env:HTTP_ADDR = "127.0.0.1:9090"; go run ./cmd/api`.
В Bash: `HTTP_ADDR=127.0.0.1:9090 go run ./cmd/api`.

Для API запуска нужны ANALYZER_DATABASE_URL, применённые миграции и
ANALYZER_VERSIONS. В Bash JSON задаётся с внешними одинарными кавычками:
`export ANALYZER_VERSIONS='{"schema":"mvp-contract-1.1.0","methodology":"rhs-mvp-1.1.0","scoring_config":"rhs-mvp-1.1.0","collector_profile":"github-mvp-1.1.0","secret_rules":"secrets-mvp-1.0.0","recommendation_catalog":"recommendations-mvp-1.0.0","explanation_schema":"catalog-selection-1.0.0","prompt":"catalog-selection-1.0.0"}'`.
Укажите версии фактически выбранных правил. API фиксирует их при принятии и
не заменяет текущими при чтении. Docker Compose передаёт эти настройки API;
вне Compose их задаёт разработчик, миграции применяются через Goose вручную.

Отправка резервируется до GitHub: повтор owner/request_id не создаёт job.
Повтор ещё проверяемой отправки даёт SUBMISSION_PENDING, смена URL —
SUBMISSION_CHANGED. Чтение failed возвращает HTTP 200; ошибка чтения —
отдельную HTTP-ошибку. Используются общий PUBLIC_ORIGIN и AUTH_REQUEST_TIMEOUT.
Текущий Auth подтверждает гостя; account cookie без подтверждения возвращает
SESSION_EXPIRED и не превращается в гостевой запуск.
В этой ветке ещё нет исполнителя #18476 и HTTP чтения Report: новый анализ
остаётся queued до подключения исполнителя. Для FR-28 sources/reasons,
выборки и ограничения профиля передаются существующим Report-контрактом;
API статуса не выдаёт вымышленные результаты сбора.

Логи пишутся в stdout в JSON. Логгер передаётся через контекст:
`logger.With(ctx, log)` и `logger.From(ctx)`. Если его нет в контексте,
`From` возвращает `slog.Default()`.

## Auth Service

gRPC-сервис по контракту [auth.proto](../../contracts/auth/v1/auth.proto):
AuthService (Register, Login, CreateGuestSession, GetSessionContext, ResolveContext, Logout) и
стандартный health-check `grpc.health.v1.Health`. Сейчас работают
CreateGuestSession, GetSessionContext и гостевая часть ResolveContext; Register, Login и Logout
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
   goose -dir migrations/auth -table goose_auth_version postgres "$AUTH_DATABASE_URL" up
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
