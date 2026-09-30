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

- [OpenAPI текущего каркаса](api/openapi.json)
- [Контракт backend API для согласования](../../docs/api/backend-integration.md)

## Структура

```text
cmd/api/                   Точка входа и обработка сигналов
internal/app/              Сборка приложения и жизненный цикл HTTP-сервера
internal/api/              HTTP-маршруты, ответы, request ID и журнал запросов
internal/config/           Чтение и валидация переменных окружения
internal/models/           Базовые модели и статусы анализа
internal/services/         Место для прикладных контрактов и сценариев
internal/adapters/github/  Место для адаптера GitHub API Service
internal/analyzers/        Место для модулей Analysis Service
api/openapi.json           Машиночитаемое описание доступных маршрутов
```

Пакеты `services`, `adapters/github` и `analyzers` пока содержат документацию
об их ответственности. Они не подключены к API Service. Схема дальнейшего
развития описана в [архитектурной заметке](../../docs/architecture/backend-foundation.md).

## Проверки и сборка

Выполняйте из `project/backend`:

```sh
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

`gofmt -l .` должен вывести пустой список. Исправление форматирования:
`gofmt -w cmd internal`. `go vet` используется как базовый статический анализатор.

Тесты проверяют HTTP-контракт, совпадение request ID в ответе и логах,
параметры окружения, реальный HTTP-сокет, HEAD, остановку сервера и занятый порт.
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
