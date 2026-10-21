# Поля common

JSON Schema: [schemas.json](../../contracts/common/v1/schemas.json).
Protobuf: [common.proto](../../contracts/common/v1/common.proto).

Здесь описаны поля моделей, а не автоматическое преобразование protobuf в JSON.
HTTP публикует только модели, на которые ссылается [OpenAPI Gateway](../../contracts/gateway/v1/openapi.json).
Внутренние контексты, токены, команды и Snapshot не принимаются от браузера.
В protobuf optional хранит наличие скаляра; поля-сообщения имеют собственный признак наличия.
В JSON обязательное поле присутствует даже при null; неизвестные списки и пустые списки различаются по состоянию источника.

## Owner

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Owner.kind` | `string; account, guest` | `string` #1 | да / нет | Неизменяемый вид владельца. | auth.principals.kind |
| `Owner.id` | `string (uuid)` | `string` #2 | да / нет | Серверный principal_id; никогда не принимается от браузера. | analyzer.analyses.owner_id |

## VerifiedContext

Минимум один контекст действителен. Для нового запуска приоритет аккаунту; чтение допускает исходный guest при одновременном account. Только доверенные внутренние вызовы.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `VerifiedContext.user_id` | `string (uuid) / null` | `optional string` #1 | да / да | Действующий аккаунт, проверен Auth. | — |
| `VerifiedContext.guest_session_id` | `string (uuid) / null` | `optional string` #2 | да / да | Одновременно действующий исходный гостевой контекст. | — |
| `VerifiedContext.account_expires_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #3 | да / да | Фиксированный срок аккаунта в UTC; отсутствует, если user_id отсутствует. | — |
| `VerifiedContext.guest_expires_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #4 | да / да | Срок исходной гостевой сессии в UTC; отсутствует, если guest_session_id отсутствует. | — |

## FieldError

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `FieldError.field` | `string` | `string` #1 | да / нет | repository_url/login/password/request_id | — |
| `FieldError.code` | `string` | `string` #2 | да / нет | Стабильный код первой ошибки поля. | — |
| `FieldError.message` | `string` | `string` #3 | да / нет | Безопасное сообщение; не содержит введённый пароль. | — |

## ApiError

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ApiError.code` | `string; VALIDATION_FAILED, URL_REQUIRED, URL_TOO_LONG, INVALID_REPOSITORY_URL, UNSUPPORTED_PLATFORM, REPOSITORY_NOT_FOUND_OR_PRIVATE, REPOSITORY_CHECK_FAILED, SUBMISSION_CHANGED, SUBMISSION_PENDING, SESSION_REQUIRED, SESSION_EXPIRED, LOGIN_REQUIRED, INVALID_CREDENTIALS, LOGIN_TAKEN, ANALYSIS_NOT_FOUND, REPORT_NOT_FOUND, UPSTREAM_RATE_LIMITED, UPSTREAM_TIMEOUT, BUDGET_EXCEEDED, ANALYSIS_FAILED, REPORT_SAVE_FAILED, CSRF_REJECTED, INTERNAL_ERROR, METHOD_NOT_ALLOWED, NOT_IMPLEMENTED, NOT_FOUND` | `string` #1 | да / нет | Стабильный UPPER_SNAKE_CASE код; см. errors.md. | — |
| `ApiError.message` | `string` | `string` #2 | да / нет | Безопасное пользовательское сообщение. | — |
| `ApiError.trace_id` | `string` | `string` #3 | да / нет | Диагностический идентификатор; не ключ идемпотентности. | — |
| `ApiError.field_errors` | `array<FieldError>` | `repeated FieldError` #4 | да / нет | Ошибки отдельных полей; пустой список при общей ошибке. | — |

## ErrorEnvelope

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ErrorEnvelope.error` | `ApiError` | `ApiError` #1 | да / нет | Безопасная структурированная ошибка ApiError; null при отсутствии ошибки. | — |

## Empty

Пустое сообщение: полей нет. Используется там, где RPC не принимает данные или не возвращает тело.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|

## Repository

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Repository.platform` | `string; github` | `string` #1 | да / нет | Поддерживаемая платформа репозитория: github. | analyzer.analyses.repository.platform |
| `Repository.github_id` | `string` | `string` #2 | да / нет | Постоянный ID GitHub как строка. | analyzer.analyses.repository.github_id |
| `Repository.canonical_url` | `string (uri)` | `string` #3 | да / нет | https://github.com/{owner}/{repository}, lowercase, без .git и / на конце. | analyzer.analyses.canonical_url |
| `Repository.owner` | `string` | `string` #4 | да / нет | Подтверждённый владелец GitHub; не владелец анализа. | analyzer.analyses.repository / analyzer.snapshots.payload.repository / report.reports.payload.repository → owner |
| `Repository.name` | `string` | `string` #5 | да / нет | Подтверждённое отображаемое имя GitHub. | analyzer.analyses.repository / analyzer.snapshots.payload.repository / report.reports.payload.repository → name |
| `Repository.default_branch` | `string / null` | `optional string` #6 | да / да | Неизвестная ветка = null. | analyzer.analyses.repository / analyzer.snapshots.payload.repository / report.reports.payload.repository → default_branch |
| `Repository.commit_sha` | `string / null` | `optional string` #7 | да / да | Зафиксированный SHA; неизвестный/пустой репозиторий=null. | analyzer.analyses.repository / analyzer.snapshots.payload.repository / report.reports.payload.repository → commit_sha |

## Window

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Window.from` | `string (date-time)` | `google.protobuf.Timestamp` #1 | да / нет | Включённая нижняя граница, t−140 дней. | analyzer.analyses.window_from/window_until / analyzer.snapshots.payload.window / report.reports.payload.window → from |
| `Window.until` | `string (date-time)` | `google.protobuf.Timestamp` #2 | да / нет | Исключённая верхняя граница, t. UTC, from < until. | analyzer.analyses.window_from/window_until / analyzer.snapshots.payload.window / report.reports.payload.window → until |

## Versions

Версии фиксируются в момент принятия и сохраняются, не подставляются текущие при чтении.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Versions.schema` | `string` | `string` #1 | да / нет | mvp-contract-1.1.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → schema |
| `Versions.methodology` | `string` | `string` #2 | да / нет | rhs-mvp-1.1.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → methodology |
| `Versions.scoring_config` | `string` | `string` #3 | да / нет | rhs-mvp-1.1.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → scoring_config |
| `Versions.collector_profile` | `string` | `string` #4 | да / нет | github-mvp-1.1.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → collector_profile |
| `Versions.secret_rules` | `string` | `string` #5 | да / нет | secrets-mvp-1.0.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → secret_rules |
| `Versions.recommendation_catalog` | `string` | `string` #6 | да / нет | recommendations-mvp-1.0.0 | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → recommendation_catalog |
| `Versions.explanation_schema` | `string` | `string` #7 | да / нет | Версия конечной схемы выбора вариантов. | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → explanation_schema |
| `Versions.prompt` | `string` | `string` #8 | да / нет | Версия параметров выбора объяснений. | analyzer.analyses.versions / analyzer.snapshots.payload.versions / report.reports.payload.versions → prompt |

## Reason

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Reason.code` | `string` | `string` #1 | да / нет | rate_limited/timeout/insufficient_permissions/unsupported_format/volume_limit/incomplete_sample/empty_sample/not_applicable/unknown и конкретные budget codes. | analyzer.snapshots.payload / report.reports.payload (reason) → code |
| `Reason.message` | `string` | `string` #2 | да / нет | Безопасное объяснение причины. | analyzer.snapshots.payload / report.reports.payload (reason) → message |
| `Reason.cannot_conclude` | `string` | `string` #3 | да / нет | Какой вывод нельзя сделать при этом ограничении. | analyzer.snapshots.payload / report.reports.payload (reason) → cannot_conclude |

## Sample

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Sample.collected` | `integer / null` | `optional int32` #1 | да / да | Известное число полученных объектов; неизвестно=null. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → collected |
| `Sample.total` | `integer / null` | `optional int32` #2 | да / да | Полный размер только если известен. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → total |
| `Sample.numerator` | `integer / null` | `optional int32` #3 | да / да | Известный числитель доли; null при отсутствии данных. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → numerator |
| `Sample.denominator` | `integer / null` | `optional int32` #4 | да / да | Известный знаменатель доли; null при отсутствии данных. Пустая выборка не равна успешной проверке. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → denominator |
| `Sample.excluded` | `integer / null` | `optional int32` #5 | да / да | Число исключённых элементов; null, если число не установлено. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → excluded |
| `Sample.excluded_reason` | `string / null` | `optional string` #6 | да / да | Причина исключения элементов; null, если исключений нет или причина неприменима. | analyzer.snapshots.payload.inputs[].sample / report.reports.payload.categories[].metrics[].input.sample → excluded_reason |

## SourceState

partial/unavailable/not_applicable требуют reason; complete не означает применимость любой формулы.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `SourceState.source` | `string; git, github, osv` | `string` #1 | да / нет | Идентификатор источника данных: Git, GitHub API, OSV либо соответствующий адаптер. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → source |
| `SourceState.scope` | `string` | `string` #2 | да / нет | tree/activity_history/secret_history/actions/branch_rules/dependencies/osv. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → scope |
| `SourceState.status` | `string; complete, partial, unavailable, not_applicable` | `string` #3 | да / нет | Полнота данных источника: complete, partial, unavailable или not_applicable. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → status |
| `SourceState.reason` | `Reason / null` | `Reason` #4 | да / да | Причина partial/unavailable/not_applicable и границы допустимых выводов; null, если не требуется. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → reason |
| `SourceState.sample` | `Sample` | `Sample` #5 | да / нет | Фактический объём и границы выборки, числитель, знаменатель и исключения. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → sample |
| `SourceState.obtained_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #6 | да / да | Время получения этих данных от источника в UTC. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → obtained_at |
| `SourceState.fingerprint` | `string / null` | `optional string` #7 | да / да | Отпечаток сохранённых входов. Сам по себе не доказывает актуальность внешнего источника. | analyzer.snapshots.payload.sources[] / report.reports.payload.sources[] → fingerprint |

## GitRef

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `GitRef.name` | `string` | `string` #1 | да / нет | Полный refs/heads/... либо refs/tags/... | analyzer.snapshots.payload.refs[] → name |
| `GitRef.object_sha` | `string` | `string` #2 | да / нет | Объявленный SHA объекта; для annotated tag хранится tag object. | analyzer.snapshots.payload.refs[] → object_sha |

## DependencyOrigin

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `DependencyOrigin.manifest_path` | `string` | `string` #1 | да / нет | Путь корневого манифеста либо lock-файла. | analyzer.snapshot_dependencies.origins[] → manifest_path |
| `DependencyOrigin.version_source` | `string` | `string` #2 | да / нет | npm lock / Maven declaration либо dependencyManagement с локальными properties / Go require. | analyzer.snapshot_dependencies.origins[] → version_source |

## Dependency

Уникальность зависимости по (ecosystem, package, version). Общий лимит 20 на три экосистемы.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Dependency.ecosystem` | `string; npm, Maven, Go` | `string` #1 | да / нет | Экосистема пакета с точным регистром: npm, Maven или Go. | analyzer.snapshot_dependencies.ecosystem |
| `Dependency.package` | `string` | `string` #2 | да / нет | npm: scope/name; Maven: groupId:artifactId; Go: полный module path. | analyzer.snapshot_dependencies.package |
| `Dependency.version` | `string` | `string` #3 | да / нет | Конкретная разрешённая либо объявленная версия. | analyzer.snapshot_dependencies.version |
| `Dependency.manifest_path` | `string` | `string` #4 | да / нет | Основное основание. | analyzer.snapshot_dependencies.origins |
| `Dependency.version_source` | `string` | `string` #5 | да / нет | Источник версии, декларация не означает разрешённый состав сборки. | analyzer.snapshot_dependencies.origins |
| `Dependency.origins` | `array<DependencyOrigin>` | `repeated DependencyOrigin` #6 | да / нет | Все основания при дедупликации одного ecosystem/package/version. | analyzer.snapshot_dependencies + analyzer.snapshots.payload.dependencies[] / report.reports.payload.dependencies[] → origins |

## Vulnerability

Дедупликация по vulnerability_id/ecosystem/package/version; исходные подтверждённые находки сохраняются даже при неполном агрегате.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Vulnerability.vulnerability_id` | `string` | `string` #1 | да / нет | Стабильный идентификатор OSV. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → vulnerability_id |
| `Vulnerability.ecosystem` | `string; npm, Maven, Go` | `string` #2 | да / нет | Экосистема пакета с точным регистром: npm, Maven или Go. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → ecosystem |
| `Vulnerability.package` | `string` | `string` #3 | да / нет | Идентификатор пакета в его экосистеме; Maven использует groupId:artifactId. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → package |
| `Vulnerability.version` | `string` | `string` #4 | да / нет | Конкретная версия пакета, используемая для запроса OSV и идентичности зависимости. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → version |
| `Vulnerability.severity` | `string; critical, high, medium, low, None / null` | `optional string` #5 | да / да | Неизвестный уровень = null; полный SEC_VULN_RISK тогда N/A. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → severity |
| `Vulnerability.obtained_at` | `string (date-time)` | `google.protobuf.Timestamp` #6 | да / нет | Время получения этих данных от источника в UTC. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → obtained_at |
| `Vulnerability.evidence_url` | `string (uri) / null` | `optional string` #7 | да / да | Ссылка на первичное описание уязвимости; null, если ссылка недоступна. | analyzer.snapshot_vulnerabilities / analyzer.snapshots.payload.vulnerabilities[] / report.reports.payload.findings[].vulnerabilities[] → evidence_url |

## Evidence

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Evidence.id` | `string` | `string` #1 | да / нет | Стабильный в пределах снимка. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → id |
| `Evidence.source` | `string; git, github, osv` | `string` #2 | да / нет | Идентификатор источника данных: Git, GitHub API, OSV либо соответствующий адаптер. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → source |
| `Evidence.kind` | `string` | `string` #3 | да / нет | file/commit/secret_match/ci_run/branch_rule/dependency/advisory/aggregate | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → kind |
| `Evidence.description` | `string` | `string` #4 | да / нет | Безопасный факт, без исходной строки/значения секрета. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → description |
| `Evidence.path` | `string / null` | `optional string` #5 | да / да | Путь к файлу относительно корня репозитория; null, если доказательство не связано с файлом. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → path |
| `Evidence.revision` | `string / null` | `optional string` #6 | да / да | Ревизия Git, к которой относится доказательство; null, если неприменимо. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → revision |
| `Evidence.url` | `string (uri) / null` | `optional string` #7 | да / да | Только сохранённый абсолютный HTTPS без credentials; иначе null. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → url |
| `Evidence.run_id` | `string / null` | `optional string` #8 | да / да | Идентификатор запуска CI у провайдера; null для доказательств другого вида. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → run_id |
| `Evidence.attempt` | `integer / null` | `optional int32` #9 | да / да | Номер попытки запуска CI; null, если он неизвестен или неприменим. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → attempt |
| `Evidence.dependency` | `Dependency / null` | `Dependency` #10 | да / да | Зависимость, к которой относится доказательство; null для иных доказательств. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → dependency |
| `Evidence.vulnerability_id` | `string / null` | `optional string` #11 | да / да | Идентификатор уязвимости из источника OSV; null там, где доказательство имеет другой тип. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → vulnerability_id |
| `Evidence.secret_match_type` | `string / null` | `optional string` #12 | да / да | private_key_header либо suspicious_assignment; не действительность секрета. | analyzer.snapshots.payload.inputs[].evidence[] / report.reports.payload.categories[].metrics[].input.evidence[] → secret_match_type |

## MetricInput

Сохранённые нормализованные безопасные входы. В snapshot нет произвольного README, логов команд или значений секретов.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `MetricInput.id` | `string; ACT_COMMIT_FREQ, ACT_DAYS_SINCE_LAST, ACT_TREND, DOC_README, DOC_SUPPORT_SHARE, TEST_PRESENCE, SEC_SECRETS, SEC_VULN_RISK, SEC_SAST, CI_PRESENT, CI_SUCCESS_RATE, CI_REQUIRED_CHECKS` | `string` #1 | да / нет | Один из 12 стабильных идентификаторов метрик методики. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → id |
| `MetricInput.raw_value` | `null / boolean / number / string` | `google.protobuf.Value` #2 | да / да | Подтверждённый scalar: number/bool/string/null. 0/false не заменяют null. Секреты никогда не включаются. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → raw_value |
| `MetricInput.unit` | `string` | `string` #3 | да / нет | commits_per_week/days/ratio/bool/secret_scan/risk | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → unit |
| `MetricInput.status` | `string; complete, partial, unavailable, not_applicable` | `string` #4 | да / нет | Полнота входных данных данной метрики; не подменяется числом или булевым значением. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → status |
| `MetricInput.reason` | `Reason / null` | `Reason` #5 | да / да | Причина partial/unavailable/not_applicable и границы допустимых выводов; null, если не требуется. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → reason |
| `MetricInput.window` | `Window / null` | `Window` #6 | да / да | Точные границы окна активности [t−140 дней, t). | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → window |
| `MetricInput.sample` | `Sample` | `Sample` #7 | да / нет | Фактический объём и границы выборки, числитель, знаменатель и исключения. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → sample |
| `MetricInput.series` | `array<integer>` | `repeated int32` #8 | да / нет | Недельные счётчики для ACT_TREND, иначе []. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → series |
| `MetricInput.evidence` | `array<Evidence>` | `repeated Evidence` #9 | да / нет | Безопасные структурированные основания результата; значения секретов и исходные строки не включаются. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → evidence |
| `MetricInput.evidence_limited` | `boolean` | `bool` #10 | да / нет | true, если в ответ включена только часть собранных доказательств. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → evidence_limited |
| `MetricInput.evidence_total` | `integer / null` | `optional int32` #11 | да / да | Полное известное число доказательств; null, если полное число не установлено. | analyzer.snapshots.payload.inputs[] / report.reports.payload.categories[].metrics[].input → evidence_total |
| `MetricInput.observed_at` | `string (date-time) / null` | `google.protobuf.Timestamp` #12 | да / да | Например фактическая дата последнего коммита; неизвестно/неприменимо=null. | analyzer.snapshots.payload.inputs[].observed_at / report.reports.payload.categories[].metrics[].input.observed_at |
| `MetricInput.exclusions` | `array<ExcludedCount>` | `repeated ExcludedCount` #13 | да / нет | Отдельные количества исключений, а не одна потерянная сумма. | analyzer.snapshots.payload.inputs[].exclusions / report.reports.payload.categories[].metrics[].input.exclusions |

## FreshnessKey

Нет TTL/updated_at shortcut; новые t/окно означают новый анализ.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `FreshnessKey.owner` | `Owner` | `Owner` #1 | да / нет | Неизменяемый владелец результата: account или исходный guest; структура Owner. | — |
| `FreshnessKey.repository` | `Repository` | `Repository` #2 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | — |
| `FreshnessKey.assessment_at` | `string (date-time)` | `google.protobuf.Timestamp` #3 | да / нет | Зафиксированный момент оценки t в UTC; не обновляется при чтении результата. | — |
| `FreshnessKey.window` | `Window` | `Window` #4 | да / нет | Точные границы окна активности [t−140 дней, t). | — |
| `FreshnessKey.refs` | `array<GitRef>` | `repeated GitRef` #5 | да / нет | Зафиксированные объявленные ветки и теги Git с идентификаторами объектов. | — |
| `FreshnessKey.sources` | `array<SourceState>` | `repeated SourceState` #6 | да / нет | Состояния источников, время получения, объём выборки, причины ограничений и отпечатки. | — |
| `FreshnessKey.versions` | `Versions` | `Versions` #7 | да / нет | Версии схемы, методики, профилей, правил и параметров объяснения для воспроизводимости. | — |
| `FreshnessKey.explanation_parameters_fingerprint` | `string` | `string` #8 | да / нет | Отпечаток всех параметров выбора объяснения, влияющих на переиспользование. | — |
| `FreshnessKey.full_context_confirmed` | `boolean` | `bool` #9 | да / нет | true только при доказанной неизменности всех использованных внешних наборов. Не выводится из локального хэша ответа. | — |

## ExcludedCount

Число исключённых объектов по одной причине.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ExcludedCount.reason` | `string` | `string` #1 | да / нет | Например cancelled/skipped/neutral. | — |
| `ExcludedCount.count` | `integer` | `int32` #2 | да / нет | Подтверждённое количество. | — |

