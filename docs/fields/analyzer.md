# Поля analyzer

JSON Schema: [schemas.json](../../contracts/analyzer/v1/schemas.json).
Protobuf: [analyzer.proto](../../contracts/analyzer/v1/analyzer.proto).

Здесь описаны поля моделей, а не автоматическое преобразование protobuf в JSON.
HTTP публикует только модели, на которые ссылается [OpenAPI Gateway](../../contracts/gateway/v1/openapi.json).
Внутренние контексты, токены, команды и Snapshot не принимаются от браузера.
В protobuf optional хранит наличие скаляра; поля-сообщения имеют собственный признак наличия.
В JSON обязательное поле присутствует даже при null; неизвестные списки и пустые списки различаются по состоянию источника.

## Внутренний интерфейс Analyzer

| Операция | Команда | Результат |
|---|---|---|
| `ValidateRepository` | `ValidateRepositoryCommand` | `ValidatedRepository` |
| `CreateAnalysis` | `CreateAnalysisCommand` | `CreateAnalysisResponse` |
| `GetAnalysis` | `GetAnalysisQuery` | `AnalysisStatus` |

Эти операции не объявлены сетевыми RPC в analyzer.proto.

## ValidateRepositoryRequest

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ValidateRepositoryRequest.repository_url` | `string` | `string` #1 | да / нет | FR-01, до 2048 code points после trim U+0020; строгая проверка исходной строки. | — |

## ValidatedRepository

Публичность подтверждается данными GitHub; private/404 одинаковы, сбой источника отличается.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ValidatedRepository.repository` | `Repository` | `repopulse.common.v1.Repository` #1 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | — |
| `ValidatedRepository.moved_from` | `string (uri) / null` | `optional string` #2 | да / да | Исходный канонический адрес при подтверждённом переносе с тем же GitHub ID. | — |

## CreateAnalysisRequest

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `CreateAnalysisRequest.request_id` | `string (uuid)` | `string` #1 | да / нет | Стабильный ID пользовательской отправки; повтор после потери ответа сохраняет его. | analyzer.submissions.request_id |
| `CreateAnalysisRequest.repository_url` | `string` | `string` #2 | да / нет | Сервер независимо валидирует URL; клиент не задаёт владельца. | analyzer.submissions (вход запроса) → repository_url |

## CreateAnalysisResponse

202 accepted / 200 reused. Replay проверяется до новых обращений к изменяемым источникам; другой URL с тем же request_id → 409.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `CreateAnalysisResponse.outcome` | `string; accepted, reused` | `string` #1 | да / нет | accepted — принято новое задание; reused — выбран ранее сохранённый отчёт. | — |
| `CreateAnalysisResponse.analysis_id` | `string (uuid)` | `string` #2 | да / нет | Стабильный ID принятого анализа либо анализа переиспользуемого отчёта. | — |
| `CreateAnalysisResponse.report_id` | `string (uuid) / null` | `optional string` #3 | да / да | null для нового анализа, ID прежнего отчёта для reused. | — |
| `CreateAnalysisResponse.status` | `string; queued, running, completed, failed` | `string` #4 | да / нет | Текущее состояние принятого анализа; для reused — completed. | — |
| `CreateAnalysisResponse.repository` | `Repository` | `repopulse.common.v1.Repository` #5 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | — |

## AnalysisStatus

Никаких фиктивных процентов. Ошибка чтения статуса не меняет состояние анализа.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `AnalysisStatus.analysis_id` | `string (uuid)` | `string` #1 | да / нет | UUID анализа; используется для чтения статуса и связи со снимком и отчётом. | analyzer.analyses.id |
| `AnalysisStatus.repository` | `Repository` | `repopulse.common.v1.Repository` #2 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | analyzer.analyses (проекция) → repository |
| `AnalysisStatus.status` | `string; queued, running, completed, failed` | `string` #3 | да / нет | Сохранённое состояние: queued, running, completed или failed. | analyzer.analyses.status |
| `AnalysisStatus.stage` | `string / null` | `optional string` #4 | да / да | Подтверждённый этап: collecting_git, collecting_external, calculating, explaining, saving. null/неизвестное значение не меняет status и игнорируется UI. | analyzer.analyses.stage |
| `AnalysisStatus.created_at` | `string (date-time)` | `google.protobuf.Timestamp` #5 | да / нет | Момент создания записи в UTC. | analyzer.analyses.created_at |
| `AnalysisStatus.started_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #6 | да / да | Момент начала выполнения; null до запуска. | analyzer.analyses.started_at |
| `AnalysisStatus.completed_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #7 | да / да | Момент терминального завершения в UTC; null до completed/failed там, где допускается. | analyzer.analyses.completed_at |
| `AnalysisStatus.report_id` | `string (uuid) / null` | `optional string` #8 | да / да | Обязателен для completed, иначе null. | analyzer.analyses.report_id |
| `AnalysisStatus.error` | `ApiError / null` | `repopulse.common.v1.ApiError` #9 | да / да | Безопасная структурированная ошибка ApiError; null при отсутствии ошибки. | analyzer.analyses (проекция) → error |

## Snapshot

Внутренний неизменяемый снимок, необходимый для повторного численного расчёта без сети.
Сборка полученных входов и передача на атомарное сохранение:
[реализация #18479](../architecture/analyzer-inputs.md).

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Snapshot.snapshot_id` | `string (uuid)` | `string` #1 | да / нет | UUID сохранённого снимка входных данных анализа. | analyzer.snapshots.id |
| `Snapshot.analysis_id` | `string (uuid)` | `string` #2 | да / нет | UUID анализа; используется для чтения статуса и связи со снимком и отчётом. | analyzer.snapshots.analysis_id |
| `Snapshot.owner` | `Owner` | `repopulse.common.v1.Owner` #3 | да / нет | Неизменяемый владелец результата: account или исходный guest; структура Owner. | analyzer.snapshots.payload → owner |
| `Snapshot.repository` | `Repository` | `repopulse.common.v1.Repository` #4 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | analyzer.snapshots.payload → repository |
| `Snapshot.assessment_at` | `string (date-time)` | `google.protobuf.Timestamp` #5 | да / нет | t фиксируется при первой пользовательской отправке. | analyzer.analyses.assessment_at |
| `Snapshot.window` | `Window` | `repopulse.common.v1.Window` #6 | да / нет | Точные границы окна активности [t−140 дней, t). | analyzer.snapshots.payload → window |
| `Snapshot.refs` | `array<GitRef>` | `repeated repopulse.common.v1.GitRef` #7 | да / нет | Зафиксированные объявленные ветки и теги Git с идентификаторами объектов. | analyzer.snapshots.payload → refs |
| `Snapshot.versions` | `Versions` | `repopulse.common.v1.Versions` #8 | да / нет | Версии схемы, методики, профилей, правил и параметров объяснения для воспроизводимости. | analyzer.snapshots.payload → versions |
| `Snapshot.sources` | `array<SourceState>` | `repeated repopulse.common.v1.SourceState` #9 | да / нет | Состояния источников, время получения, объём выборки, причины ограничений и отпечатки. | analyzer.snapshots.payload → sources |
| `Snapshot.inputs` | `array<MetricInput>` | `repeated repopulse.common.v1.MetricInput` #10 | да / нет | Все 12 входных метрик с исходными значениями, выборкой и доказательствами. | analyzer.snapshots.payload → inputs |
| `Snapshot.dependencies` | `array<Dependency>` | `repeated repopulse.common.v1.Dependency` #11 | да / нет | Прямые зависимости npm/Maven/Go; дедупликация по ecosystem/package/version, общий лимит 20. | analyzer.snapshots.payload → dependencies |
| `Snapshot.vulnerabilities` | `array<Vulnerability>` | `repeated repopulse.common.v1.Vulnerability` #12 | да / нет | Известные уязвимости конкретных версий зависимостей; идентификаторы и серьёзность OSV. | analyzer.snapshots.payload → vulnerabilities |

## GetAnalysisQuery

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `GetAnalysisQuery.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `GetAnalysisQuery.analysis_id` | `string (uuid)` | `string` #2 | да / нет | UUID анализа; используется для чтения статуса и связи со снимком и отчётом. | — |

## CreateAnalysisCommand

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `CreateAnalysisCommand.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `CreateAnalysisCommand.request` | `CreateAnalysisRequest` | `CreateAnalysisRequest` #2 | да / нет | Тело исходного пользовательского запроса; владелец определяется отдельно через context. | — |

## ValidateRepositoryCommand

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ValidateRepositoryCommand.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `ValidateRepositoryCommand.request` | `ValidateRepositoryRequest` | `ValidateRepositoryRequest` #2 | да / нет | Тело исходного пользовательского запроса; владелец определяется отдельно через context. | — |

