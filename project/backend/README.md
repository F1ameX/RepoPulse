# RepoPulse Backend

Backend-каркас задачи `task-17661` на Go 1.26.0+. Использует стандартную
библиотеку: `net/http`, `log/slog`, `testing`. Внешних Go-зависимостей пока нет,
поэтому `go.sum` не требуется.

Подготовлены девять независимо запускаемых процессов. Каждый предоставляет
служебный `/health`; API Service / BFF также резервирует три маршрута API v1.
Анализ, авторизация, gRPC/Kafka и хранилища будут реализованы отдельными задачами.
PostgreSQL, Redis и Kafka для запуска каркасов не нужны.

## Быстрый старт

Из корня репозитория, одинаково в PowerShell и Bash:

```sh
cd project/backend
go run ./cmd/api
```

Сервер слушает `127.0.0.1:8080`. Во втором терминале:

```sh
curl http://127.0.0.1:8080/health
```

В Windows PowerShell можно использовать `curl.exe` или:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

Ожидаемый ответ: HTTP `200`, `Content-Type: application/json; charset=utf-8`,
тело `{"status":"ok"}` и заголовок `X-Request-ID`.
`/health` проверяет доступность HTTP-процесса; проверок готовности будущих
внешних сервисов в нём нет.

Остановка: `Ctrl+C` или `SIGTERM`. Сервер прекращает приём новых соединений
и ждёт завершения активных запросов до `SHUTDOWN_TIMEOUT`. Если срок истёк,
соединения закрываются, процесс завершается с ошибкой.

## Девять сервисов

Команды выполняются из `project/backend`, каждый сервис — в своём терминале:

| Сервис | Команда | HTTP-адрес по умолчанию |
| --- | --- | --- |
| API Service / BFF | `go run ./cmd/api` | `127.0.0.1:8080` |
| Auth Service | `go run ./cmd/auth` | `127.0.0.1:8081` |
| Analysis Orchestrator | `go run ./cmd/orchestrator` | `127.0.0.1:8082` |
| GitHub API Service | `go run ./cmd/github` | `127.0.0.1:8083` |
| Repository Sandbox Service | `go run ./cmd/sandbox` | `127.0.0.1:8084` |
| Analysis Service | `go run ./cmd/analysis` | `127.0.0.1:8085` |
| Scoring Service | `go run ./cmd/scoring` | `127.0.0.1:8086` |
| Recommendation Service | `go run ./cmd/recommendation` | `127.0.0.1:8087` |
| Report Service | `go run ./cmd/report` | `127.0.0.1:8088` |

Порты различаются, поэтому сервисы можно запускать одновременно без настройки
окружения. Общий `HTTP_ADDR` переопределяет адрес только текущего процесса;
задавайте разные адреса для процессов, если используете эту переменную.

Например, после запуска Auth Service:

```sh
curl http://127.0.0.1:8081/health
```

У восьми внутренних сервисов HTTP-интерфейс содержит только технический
`GET /health` (также HEAD). `/api/v1/*` там возвращает `404`. Это служебный
HTTP-порт для контроля процесса; межсервисные gRPC/Kafka-порты ещё не открываются.
Назначение сервисов и будущие адаптеры описаны в [матрице сервисов](docs/services.md).

## Конфигурация

Приложение читает переменные окружения процесса. `.env.example` — справочник
значений; Go-приложение **не загружает `.env` автоматически**. Если переменная
не задана, применяется значение по умолчанию. Явно пустые и некорректные
значения приводят к ошибке при запуске.

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `HTTP_ADDR` | Из таблицы сервисов | Адрес и порт HTTP, порт от 1 до 65535 |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Время чтения заголовков |
| `HTTP_READ_TIMEOUT` | `15s` | Время чтения запроса |
| `HTTP_WRITE_TIMEOUT` | `30s` | Время записи ответа |
| `HTTP_IDLE_TIMEOUT` | `60s` | Время ожидания на keep-alive соединении |
| `SHUTDOWN_TIMEOUT` | `10s` | Время корректной остановки |

Все интервалы должны быть положительными, с единицами Go duration (`500ms`,
`5s`, `1m`). Для прослушивания всех интерфейсов укажите `HTTP_ADDR=0.0.0.0:8080`.

PowerShell:

```powershell
$env:HTTP_ADDR = '127.0.0.1:9090'
$env:LOG_LEVEL = 'debug'
go run ./cmd/api
```

Bash:

```sh
HTTP_ADDR=127.0.0.1:9090 LOG_LEVEL=debug go run ./cmd/api
```

Логи сервера пишутся в stdout в JSON. Запись HTTP-запроса содержит `service`,
`request_id`, `method`, `path`, `status`, `duration_ms`. ID генерируется сервером
для каждого запроса и возвращается в `X-Request-ID`; входящий ID не используется.
Тела запросов и query-параметры не логируются.

Логгер передаётся через контекст: `logger.With(ctx, log)` сохраняет его,
`logger.From(ctx)` извлекает. Утилита находится в `libs/logger` и использует
закрытый тип ключа. Если логгер отсутствует или равен `nil`, возвращается
`slog.Default()`, чтобы отсутствие логгера в контексте не прерывало запрос.
В HTTP-обработчиках контекст содержит логгер с `request_id` текущего запроса.
`http.Server.BaseContext` наследует значения контекста приложения, сохраняя
возможность завершить активные запросы при graceful shutdown.

## API

Пути API v1 ниже доступны только у API Service / BFF на порту `8080`.

| Метод | Путь | Текущее поведение |
| --- | --- | --- |
| `GET` | `/health` | `200`, `{"status":"ok"}` |
| `POST` | `/api/v1/analyses` | `501 NOT_IMPLEMENTED` |
| `GET` | `/api/v1/analyses/{analysis_id}` | `501 NOT_IMPLEMENTED` |
| `GET` | `/api/v1/reports/{report_id}` | `501 NOT_IMPLEMENTED` |

GET-маршруты также поддерживают HEAD. Неизвестный маршрут возвращает `404`,
неподдерживаемый метод — `405` с заголовком `Allow`. Ошибки этих обработчиков
имеют единый JSON-формат:

```json
{
  "error": {
    "code": "NOT_IMPLEMENTED",
    "message": "Analysis and report services are not implemented yet",
    "request_id": "generated-request-id"
  }
}
```

Зарезервированные обработчики не читают тело запроса, не создают анализы и
не возвращают демонстрационные отчёты. Их `501` сохраняет этот факт явным для
frontend до подключения доменных сервисов и проверки JWT.

- [OpenAPI текущего каркаса](docs/openapi.json)
- [Контракт backend API для согласования](../../docs/api/backend-integration.md)

## Структура

```text
cmd/<service>/                         api, auth, orchestrator, github, sandbox,
                                       analysis, scoring, recommendation, report
  main.go                               Отдельная точка входа каждого процесса
  k8s/                                  Место для конфигураций развёртывания
database/                               Место для схем, пользователей и миграций
docs/openapi.json                       HTTP-контракт
internal/<service>/                    Девять каталогов сервисов
    adapter/in/http/                    Health; у api также внешние маршруты
    adapter/in/{grpc,kafka}/            Будущие входящие доменные адаптеры
    adapter/out/<name>/                 Будущие клиенты и хранилища сервиса
    app/
      app.go                            Запуск и остановка HTTP-сервера
      config.go                         Чтение и валидация окружения
      init.go                           Инициализация HTTP-сервера
    model/                              Модели сервиса
    service/ports.go                    Место для портов и сценариев сервиса
internal/pkg/
  command/                              Общие сигналы и инициализация логгера
  config/                               Общие настройки процесса и валидация
  httpserver/                           Жизненный цикл HTTP-сервера
  adapter/
    in/http/                            Общие health и request logging
    in/kafka/                           Место для общих Kafka consumers
    out/{kafka,repository}/             Место для общих producers и хранилищ
libs/logger/                            Работа с логгером в контексте
proto/                                  Место для исходных .proto-контрактов
pkg/proto/                              Место для сгенерированного Go-кода
```

Во всех девяти сервисах есть `adapter`, `app`, `model` и `service`.
Входящие адаптеры HTTP, gRPC и Kafka размещаются в `adapter/in`, исходящие
клиенты, Kafka producers и репозитории — в `adapter/out` по мере реализации
соответствующего транспорта. Зарезервированы только адаптеры, соответствующие
роли сервиса; например, у Auth нет Kafka, у BFF нет репозитория БД.
Порты объявляются в `service/ports.go`, а
реализации сценариев — в `service/<name>`.

Общая техническая реализация находится в `internal/pkg`, каждый `app`
задаёт имя, адрес по умолчанию и подключает свой входящий HTTP-адаптер.
Будущие доменные адаптеры, порты, анализаторы и инфраструктурные каталоги
пока содержат документацию или `.gitkeep`. Доменные модели
анализа принадлежат Orchestrator, клиент GitHub — GitHub API Service,
анализаторы — Analysis Service. Схема дальнейшего развития описана в
[архитектурной заметке](../../docs/architecture/backend-foundation.md).

## Проверки и сборка

Выполняйте из `project/backend`:

```sh
gofmt -l .
go vet ./...
go test ./...
go build ./...
go build -o bin/ ./cmd/...
```

`gofmt -l .` должен вывести пустой список. Исправление форматирования:
`gofmt -w cmd internal libs`. `go vet` используется как базовый статический анализатор.

Тесты проверяют HTTP-контракт, совпадение request ID в ответе и логах,
параметры окружения, реальный HTTP-сокет, HEAD, остановку сервера и занятый порт.
Отдельно проверяется сохранение логгера в дочерних контекстах, возврат
`slog.Default()` при его отсутствии, передача логгера в HTTP-запрос и изоляция
`request_id` от родительского контекста.
Интеграционный тест одновременно запускает все девять приложений на реальных
локальных сокетах, проверяет их настройки, `/health`, изоляцию внешнего API
и остановку по отмене контекста. Во время теста используются свободные порты.
В CI дополнительно выполняется `go test -race -coverprofile=coverage.out ./...`
на Ubuntu. Для локального `-race` нужен доступный C-компилятор; обычные тесты
и сборка его не требуют.

Для получения исполняемого файла:

```powershell
# Windows
go build -o bin/repopulse-api.exe ./cmd/api
./bin/repopulse-api.exe
```

```sh
# Linux / macOS
go build -o bin/repopulse-api ./cmd/api
./bin/repopulse-api
```

Команда `go build -o bin/ ./cmd/...` собирает сразу девять исполняемых файлов
(`api`, `auth`, `orchestrator`, `github`, `sandbox`, `analysis`, `scoring`,
`recommendation`, `report`; в Windows — с суффиксом `.exe`).

Workflow [Backend](../../.github/workflows/backend.yml) запускается на push и
pull request при изменениях backend или самого workflow; доступен ручной запуск.
