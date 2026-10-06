# RepoPulse backend

Каркас на Go 1.26 со стандартной библиотекой. API предоставляет `GET /health`
и заготовки маршрутов анализа и отчёта. Остальные восемь процессов пока
инициализируют логгер и ждут сигнала завершения; бизнес-логика не реализована.

## Структура

```text
cmd/<service>/main.go        точка входа каждого сервиса
internal/<service>/
  adapter/                  внешние взаимодействия
  app/                      инициализация и запуск
  domain/                   модели и бизнес-логика
internal/pkg/command/       общая настройка логирования и обработки сигналов
internal/pkg/logger/        With/From для логгера в context.Context
```

| Каталог сервиса | Назначение |
| --- | --- |
| `api` | HTTP API для frontend |
| `auth` | Аутентификация |
| `orchestrator` | Управление анализом |
| `github` | Адаптер платформы GitHub |
| `sandbox` | Подготовка репозитория |
| `analysis` | Анализаторы |
| `scoring` | Расчёт оценок |
| `recommendation` | Рекомендации |
| `report` | Сборка отчёта |

## Локальный запуск

Из корня репозитория:

```sh
cd project/backend
go run ./cmd/api
```

В другом терминале:

```sh
curl http://127.0.0.1:8080/health
# {"status":"ok"}
```

Остановка — `Ctrl+C`. Для запуска другого каркаса замените `api` именем
из таблицы, например `go run ./cmd/analysis`. Эти процессы пока не открывают порты.
API запускается самостоятельно, без остальных сервисов, БД и внешних систем.

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

## Основа API

| Метод и путь | Текущее поведение |
| --- | --- |
| `GET /health` | `200 {"status":"ok"}` — процесс работает |
| `POST /api/v1/analyses` | `501 NOT_IMPLEMENTED` |
| `GET /api/v1/analyses/{analysis_id}` | `501 NOT_IMPLEMENTED` |
| `GET /api/v1/reports/{report_id}` | `501 NOT_IMPLEMENTED` |

Для интеграции frontend использует HTTP/JSON через API. При раздельном
локальном запуске его dev proxy должен направлять `/api` в API
(по умолчанию `http://127.0.0.1:8080`). Форматы запросов и ответов уже
зафиксированы в [контрактах MVP](api/README.md); подключение этих контрактов
к маршрутам и frontend выполняется при реализации.

## Проверки

Из `project/backend`:

```sh
gofmt -l .
go vet ./...
go test ./...
go build -o bin/ ./cmd/...
```

`gofmt -l .` не должен выводить файлов. Сборка создаёт девять бинарников
в `bin/`. CI выполняет те же проверки и запускает тесты с `-race`
на Ubuntu.
