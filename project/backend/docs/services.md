# Каркасы девяти сервисов RepoPulse

Каждая строка соответствует отдельному исполняемому процессу и каталогу
`internal/<service>`. У всех есть `app/{app,config,init}.go`, `model`,
`service/ports.go`, входящий служебный HTTP-адаптер и каталог
`cmd/<service>/k8s` для будущих конфигураций развёртывания.

| Сервис | Точка входа | HTTP-порт | Назначение |
| --- | --- | --- | --- |
| API Service / BFF | `cmd/api/main.go` | 8080 | Внешний HTTP API, проверка JWT и маршрутизация запросов к доменным сервисам. |
| Auth Service | `cmd/auth/main.go` | 8081 | Пользователи, пароли, токены и refresh-сессии. |
| Analysis Orchestrator | `cmd/orchestrator/main.go` | 8082 | Состояние анализа, последовательность этапов, retry и идемпотентность. |
| GitHub API Service | `cmd/github/main.go` | 8083 | Сбор и нормализация platform-specific данных GitHub и сохранение снимков. |
| Repository Sandbox Service | `cmd/sandbox/main.go` | 8084 | Изолированное клонирование, подготовка репозитория и управление gVisor sandbox. |
| Analysis Service | `cmd/analysis/main.go` | 8085 | Технические анализаторы, SonarScanner и сохранение AnalysisSnapshot. |
| Scoring Service | `cmd/scoring/main.go` | 8086 | Расчёт Repo Health Score на основе AnalysisSnapshot. |
| Recommendation Service | `cmd/recommendation/main.go` | 8087 | Формирование рекомендаций по AnalysisSnapshot и ScoringResult. |
| Report Service | `cmd/report/main.go` | 8088 | Генерация, хранение и выдача immutable отчётов с проверкой доступа. |

Все HTTP-серверы по умолчанию слушают только `127.0.0.1`. Адрес можно
переопределить через `HTTP_ADDR` отдельно для каждого процесса. Логи содержат
фиксированное имя сервиса в поле `service`. На всех девяти портах
`GET /health` возвращает `200 {"status":"ok"}` и `X-Request-ID`.

Только BFF имеет маршруты `/api/v1/analyses`,
`/api/v1/analyses/{analysis_id}` и `/api/v1/reports/{report_id}`; они пока
возвращают `501 NOT_IMPLEMENTED`. На HTTP-портах внутренних сервисов эти
пути отсутствуют. Health проверяет работоспособность процесса, а не готовность
ещё не подключённых доменных зависимостей.

## Зарезервированные доменные адаптеры

Сейчас работает `adapter/in/http`. Остальные адаптеры содержат документацию
или `.gitkeep`; они определяют места для будущей реализации.

| Сервис | Входящие доменные адаптеры | Исходящие адаптеры |
| --- | --- | --- |
| `api` | HTTP API реализован как заглушки | `out/auth`, `out/orchestrator`, `out/report` |
| `auth` | `in/grpc` | `out/repository` |
| `orchestrator` | `in/grpc`, `in/kafka` | `out/github`, `out/report`, `out/kafka`, `out/repository`, `out/redis` |
| `github` | `in/grpc`, `in/kafka` | `out/github`, `out/kafka`, `out/repository` |
| `sandbox` | `in/kafka` | `out/gvisor`, `out/kafka` |
| `analysis` | `in/grpc`, `in/kafka` | `out/github`, `out/sonarqube`, `out/kafka`, `out/repository` |
| `scoring` | `in/grpc`, `in/kafka` | `out/analysis`, `out/llm`, `out/kafka`, `out/repository` |
| `recommendation` | `in/grpc`, `in/kafka` | `out/analysis`, `out/scoring`, `out/llm`, `out/kafka`, `out/repository` |
| `report` | `in/grpc`, `in/kafka` | `out/github`, `out/analysis`, `out/scoring`, `out/recommendation`, `out/kafka`, `out/repository`, `out/redis` |

Назначение общих имён: `repository` — БД самого сервиса, `redis` — кэш,
`kafka` — обмен командами и событиями, `llm` — AI/LLM provider.
Остальные исходящие адаптеры названы по целевому сервису или инструменту.
BFF не подключается к БД; Auth не использует Kafka. Sandbox отвечает за
изоляцию репозиториев и не получает доменное хранилище других сервисов.

## Граница реализации

Запускаются HTTP-процессы, конфигурация, JSON-логгер и обработка сигналов.
Доменные gRPC-серверы, Kafka consumers/producers, БД, gVisor, SonarQube и LLM
ещё не подключены. Каркасы не выполняют анализ и не имитируют его результаты.

Общий технический код расположен в `internal/pkg/{command,config,httpserver}`
и `internal/pkg/adapter/in/http`. Конкретные сервисы подключают его через свои
`app` и `adapter/in/http`; доменная логика остаётся в сервисах-владельцах.
`libs/logger` обеспечивает передачу логгера через контекст.
