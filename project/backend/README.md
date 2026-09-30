# RepoPulse Backend

Backend-каркас задачи `task-17661` на Go 1.26.0+. Использует стандартную
библиотеку: `net/http`, `log/slog`, `testing`. Внешних Go-зависимостей пока нет,
поэтому `go.sum` не требуется.

Сейчас запускается один процесс API Service / BFF. Он предоставляет `/health`
и резервирует три маршрута API v1. Анализ, авторизация, gRPC-клиенты и хранилища
будут реализованы отдельными задачами. PostgreSQL, Redis и Kafka для запуска
этого каркаса не нужны.

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

## Конфигурация

Приложение читает переменные окружения процесса. `.env.example` — справочник
значений; Go-приложение **не загружает `.env` автоматически**. Если переменная
не задана, применяется значение по умолчанию. Явно пустые и некорректные
значения приводят к ошибке при запуске.

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `HTTP_ADDR` | `127.0.0.1:8080` | Адрес и порт HTTP, порт от 1 до 65535 |
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
cmd/api/
  main.go                               Точка входа и обработка сигналов
  k8s/                                  Место для конфигураций развёртывания
database/                               Место для схем, пользователей и миграций
docs/openapi.json                       HTTP-контракт
internal/
  api/
    adapter/in/http/                    HTTP-обработчики и request logging
    adapter/out/{auth,orchestrator,report}/  Место для gRPC-клиентов BFF
    app/
      app.go                            Запуск и остановка HTTP-сервера
      config.go                         Чтение и валидация окружения
      init.go                           Инициализация HTTP-сервера
    model/                              Место для прикладных моделей API
    service/ports.go                    Место для портов и сценариев API
  orchestrator/model/                   Модели и статусы анализа
  github/adapter/out/github/            Место для клиента GitHub API
  analysis/service/analyzers/           Место для модулей анализа
  pkg/adapter/
    in/kafka/                           Место для общих Kafka consumers
    out/{kafka,repository}/             Место для общих producers и хранилищ
libs/logger/                            Работа с логгером в контексте
proto/                                  Место для исходных .proto-контрактов
pkg/proto/                              Место для сгенерированного Go-кода
```

Код группируется по сервису: `internal/<service>/{adapter,app,model,service}`.
Входящие адаптеры HTTP, gRPC и Kafka размещаются в `adapter/in`, исходящие
клиенты, Kafka producers и репозитории — в `adapter/out` по мере реализации
соответствующего транспорта. Порты объявляются в `service/ports.go`, а
реализации сценариев — в `service/<name>`.

Будущие адаптеры, порты, анализаторы и инфраструктурные каталоги пока содержат
документацию или `.gitkeep`. Они не подключены к API Service. Доменные модели
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
```

`gofmt -l .` должен вывести пустой список. Исправление форматирования:
`gofmt -w cmd internal libs`. `go vet` используется как базовый статический анализатор.

Тесты проверяют HTTP-контракт, совпадение request ID в ответе и логах,
параметры окружения, реальный HTTP-сокет, HEAD, остановку сервера и занятый порт.
Отдельно проверяется сохранение логгера в дочерних контекстах, возврат
`slog.Default()` при его отсутствии, передача логгера в HTTP-запрос и изоляция
`request_id` от родительского контекста.
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

Workflow [Backend](../../.github/workflows/backend.yml) запускается на push и
pull request при изменениях backend или самого workflow; доступен ручной запуск.
