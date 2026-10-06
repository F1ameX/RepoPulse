# Контракты RepoPulse MVP

Контракты описывают согласованный поток из [архитектуры](repopulse_architecture.md): Orchestrator запускает сбор GitHub и Sandbox через Kafka, ждёт готовности обоих источников и запускает Analysis. Analysis читает готовые данные по gRPC, получает результаты серверной задачи SonarQube и сохраняет snapshot. Далее последовательно выполняются Scoring, Recommendation и Report.

Гость может запустить анализ и посмотреть собственный результат в рамках сессии. История и PDF доступны авторизованному владельцу пользовательских отчётов. Гостевые отчёты не переносятся в аккаунт. Локальный сбор ограничен документацией, тестами, CI-конфигурациями, `.gitignore`, tracked `.env` и частотой коммитов. SonarScanner запускает Sandbox; Analysis получает результаты серверной обработки.

Все исходные контракты находятся в [project/backend/api](../../project/backend/api/README.md). Обработчики сервисов и Go-код из `.proto` пока не реализованы.

## gRPC

| Файл | Содержимое |
|---|---|
| [common.proto](../../project/backend/api/proto/repopulse/common/v1/common.proto) | `Requester`, окно UTC, репозиторий, completeness, состояния анализа и общие перечисления |
| [auth.proto](../../project/backend/api/proto/repopulse/auth/v1/auth.proto) | `CreateGuestSession`, `Register`, `Login`, `Refresh` |
| [orchestrator.proto](../../project/backend/api/proto/repopulse/orchestrator/v1/orchestrator.proto) | `StartAnalysis`, `GetAnalysisStatus`; новый анализ или существующий отчёт |
| [github.proto](../../project/backend/api/proto/repopulse/github/v1/github.proto) | `ResolveRepositoryState`, `GetGitHubDataSnapshot`; нормализованные данные GitHub и состояние их сбора |
| [sandbox.proto](../../project/backend/api/proto/repopulse/sandbox/v1/sandbox.proto) | Поток готовых данных `GetRepositoryData`, освобождение среды `ReleaseSandbox` |
| [analysis.proto](../../project/backend/api/proto/repopulse/analysis/v1/analysis.proto) | `GetAnalysisSnapshot`; metrics, findings, observations, completeness |
| [scoring.proto](../../project/backend/api/proto/repopulse/scoring/v1/scoring.proto) | `GetScoringResult`; оценки, объяснения, версии модели/prompt/schema |
| [recommendation.proto](../../project/backend/api/proto/repopulse/recommendation/v1/recommendation.proto) | `GetRecommendationResult`; рекомендации и ссылки на findings |
| [report.proto](../../project/backend/api/proto/repopulse/report/v1/report.proto) | Собственный отчёт, история, поток PDF и `FindReusableReport` |

Запуск тяжёлых этапов остаётся Kafka-командами. В gRPC Sandbox нет метода запуска сбора. Его поток содержит header, конкретные файловые факты MVP, дневные счётчики коммитов, данные `.gitignore`, ссылку на Sonar-задачу и summary семи секций. Порядок chunk и правила завершения описаны в архитектуре.

`Requester` содержит ровно одну идентичность с непустым ID. API формирует её из проверенных credentials. Отсутствующее значение `optional bool` означает неизвестный результат и отличается от подтверждённого `false`. Недоступная оценка не заменяется нулём.

## Kafka и DLQ

| Файл | Содержимое |
|---|---|
| [messages.schema.json](../../project/backend/api/kafka/messages.schema.json) | Envelope, шесть команд, события Started/Completed/Failed и три события Progress |
| [topics.json](../../project/backend/api/kafka/topics.json) | Соответствие сообщений топикам; ключ `analysis_id` |
| [dlq.schema.json](../../project/backend/api/kafka/dlq.schema.json) | Исходное сообщение, топик/partition/offset, сервис, число попыток и ошибка |
| [kafka-messages.json](../../project/backend/api/examples/kafka-messages.json) | Пример каждого типа, включая `GenerateReport` для гостя |
| [dlq.json](../../project/backend/api/examples/dlq.json) | Пример сообщения DLQ |

Envelope содержит `message_id`, `analysis_id`, `type`, `created_at`, `payload`. Данные результатов читаются по gRPC; credentials не передаются через Kafka. Retry выполняется локально в сервисе. После исчерпания попыток сообщение попадает в DLQ соответствующего сервиса; отдельного retry-топика и автоматического обработчика DLQ нет. DLQ остаётся ответственностью администраторов.

## HTTP и frontend

[openapi.json](../../project/backend/api/openapi/openapi.json) описывает 11 маршрутов: health, гостевую сессию, регистрацию, login/refresh, запуск и статус анализа, отчёт, PDF и два маршрута истории. Там же заданы запросы, ответы, ошибки, пагинация и доступ через пользовательский Bearer или гостевую cookie.

HTTP JSON использует `snake_case`. API преобразует внутренние protobuf-модели в HTTP DTO; прямой ProtoJSON не является публичным контрактом. Итоговый отчёт содержит оценки категорий, findings, рекомендации и completeness. Метрики и observations остаются в AnalysisSnapshot, подробные доказательства и структура репозитория в MVP отсутствуют.

Для frontend подготовлены примеры [отчёта](../../project/backend/api/examples/report.json), [статуса](../../project/backend/api/examples/analysis-status.json), [истории](../../project/backend/api/examples/report-list.json). UI-документация синхронизирована с HTTP-контрактом. Текущий код frontend ещё использует прежние модели и маршруты; подключение нового контракта выполняется при интеграции API.

## Ответы LLM

[scoring-result.schema.json](../../project/backend/api/llm/scoring-result.schema.json) и [recommendations.schema.json](../../project/backend/api/llm/recommendations.schema.json) задают структурированные ответы для Scoring и Recommendation. Сервисы добавляют идентификаторы, ссылки на входные snapshots и версии модели/prompt/schema после проверки ответа. Веса категорий и правила расчёта итоговой оценки эти схемы не определяют.

## Оставшиеся решения

До реализации соответствующих сервисов остаётся уточнить:

1. **Актуальность отчёта.** Согласованное сравнение только `repository.updated_at` не ограничивает возраст результата. Окно активности меняется со временем даже без новых коммитов. Для `FindReusableReport` нужно согласовать проверку SHA/версий и срок допустимого переиспользования. Текущий контракт сохраняет правило `updated_at` и поиск только у того же владельца.
2. **Публикация после сохранения результата.** При повторной доставке обработчик сначала проверяет сохранённый результат и восстанавливает публикацию события, не читая уже удалённый sandbox. Нужно конкретизировать атомарную запись результата и исходящего события для сервисов с PostgreSQL. Основа механизма: [transactional outbox](https://microservices.io/patterns/data/transactional-outbox).
3. **Развёртывание SonarQube и Sandbox.** В конфигурации запуска нужно определить постоянную БД/volumes SonarQube и Linux-среду с containerd/gVisor. См. [настройку контейнера SonarQube](https://docs.sonarsource.com/sonarqube-server/server-installation/from-docker-image/set-up-and-start-container) и [требования gVisor](https://gvisor.dev/docs/user_guide/install/).
4. **Методика оценки.** Нужно выбрать категории, веса и обработку частично доступных данных. Схемы уже допускают отсутствие оценки, но не назначают её автоматически.

Эти решения не меняют зафиксированные границы сервисов. Streaming, Redis, отдельные Scoring/Recommendation и существующие методы истории сохранены.
