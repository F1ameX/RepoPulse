# Поля report

JSON Schema: [schemas.json](../../contracts/report/v1/schemas.json).
Protobuf: [report.proto](../../contracts/report/v1/report.proto).

Здесь описаны поля моделей, а не автоматическое преобразование protobuf в JSON.
HTTP публикует только модели, на которые ссылается [OpenAPI Gateway](../../contracts/gateway/v1/openapi.json).
Внутренние контексты, токены, команды и Snapshot не принимаются от браузера.
В protobuf optional хранит наличие скаляра; поля-сообщения имеют собственный признак наличия.
В JSON обязательное поле присутствует даже при null; неизвестные списки и пустые списки различаются по состоянию источника.

## gRPC операции

| Метод | Запрос | Ответ |
|---|---|---|
| `GetReport` | `GetReportQuery` | `Report` |
| `ListReports` | `ListReportsQuery` | `HistoryPage` |
| `FindReusableReport` | `ReusableReportQuery` | `ReusableReportResult` |

## Metric

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Metric.input` | `MetricInput` | `repopulse.common.v1.MetricInput` #1 | да / нет | Сохранённый MetricInput: исходное значение, охват, ограничения и доказательства. | report.reports.payload.categories[].metrics[] → input |
| `Metric.normalized_score` | `number / null` | `optional double` #2 | да / да | 0..100 либо null; отсутствие балла объясняется reason. | report.reports.payload.categories[].metrics[] → normalized_score |
| `Metric.rule_version` | `string` | `string` #3 | да / нет | Версия применённого правила расчёта метрики. | report.reports.payload.categories[].metrics[] → rule_version |
| `Metric.rule` | `string` | `string` #4 | да / нет | Применённая формула и параметры, без пересчёта в UI. | report.reports.payload.categories[].metrics[] → rule |
| `Metric.threshold` | `number / null` | `optional double` #5 | да / да | Порог, использованный правилом; null для расчётов без порога. | report.reports.payload.categories[].metrics[] → threshold |
| `Metric.explanation` | `string` | `string` #6 | да / нет | Объяснение результата или рекомендации, связанное с сохранёнными основаниями. | report.reports.payload.categories[].metrics[] → explanation |

## Coverage

Общий total=12; category total=3/2/1/3/3. percentage=calculated/total*100. Counts are explicit and never inferred from absence.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Coverage.calculated` | `integer` | `int32` #1 | да / нет | Количество метрик с числовой нормализованной оценкой, включая нулевую. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → calculated |
| `Coverage.total` | `integer` | `int32` #2 | да / нет | Полное число метрик в области coverage: 12 для отчёта, число метрик для категории. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → total |
| `Coverage.percentage` | `number` | `double` #3 | да / нет | calculated/total*100; публичное округление half-up 1 знак. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → percentage |
| `Coverage.partial` | `integer` | `int32` #4 | да / нет | Количество метрик с частичными данными. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → partial |
| `Coverage.unavailable` | `integer` | `int32` #5 | да / нет | Количество метрик с недоступными данными. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → unavailable |
| `Coverage.not_applicable` | `integer` | `int32` #6 | да / нет | Количество метрик, для которых проверка неприменима. | report.reports.payload.coverage / report.reports.payload.categories[].coverage → not_applicable |

## Category

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Category.id` | `string; activity, documentation, testing, security, ci_cd` | `string` #1 | да / нет | Идентификатор одной из пяти категорий методики. | report.reports.payload.categories[] → id |
| `Category.score` | `number / null` | `optional double` #2 | да / да | Оценка 0..100; null при отсутствии достаточных данных. | report.reports.payload.categories[] → score |
| `Category.status` | `string; complete, partial, unavailable, not_applicable` | `string` #3 | да / нет | Полнота данных категории по состояниям её метрик. | report.reports.payload.categories[] → status |
| `Category.reason` | `Reason / null` | `repopulse.common.v1.Reason` #4 | да / да | Причина partial/unavailable/not_applicable и границы допустимых выводов; null, если не требуется. | report.reports.payload.categories[] → reason |
| `Category.coverage` | `Coverage` | `Coverage` #5 | да / нет | Число рассчитанных, частичных, недоступных и неприменимых метрик. | report.reports.payload.categories[] → coverage |
| `Category.effective_weight` | `number / null` | `optional double` #6 | да / да | Фактически перенормированный вклад категории. | report.reports.payload.categories[] → effective_weight |
| `Category.explanation` | `string` | `string` #7 | да / нет | Объяснение результата или рекомендации, связанное с сохранёнными основаниями. | report.reports.payload.categories[] → explanation |
| `Category.metrics` | `array<Metric>` | `repeated Metric` #8 | да / нет | Метрики категории в составе, установленном методикой; неизвестные значения не удаляются. | report.reports.payload.categories[] → metrics |

## Finding

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Finding.id` | `string` | `string` #1 | да / нет | Идентификатор находки, уникальный в пределах отчёта. | report.reports.payload.findings[] → id |
| `Finding.rule_id` | `string; ACT_COMMIT_FREQ, ACT_DAYS_SINCE_LAST, ACT_TREND, DOC_README, DOC_SUPPORT_SHARE, TEST_PRESENCE, SEC_SECRETS, SEC_VULN_RISK, SEC_SAST, CI_PRESENT, CI_SUCCESS_RATE, CI_REQUIRED_CHECKS` | `string` #2 | да / нет | Идентификатор правила или метрики, породившей находку. | report.reports.payload.findings[] → rule_id |
| `Finding.category` | `string; activity, documentation, testing, security, ci_cd` | `string` #3 | да / нет | Категория методики, к которой относится находка. | report.reports.payload.findings[] → category |
| `Finding.title` | `string` | `string` #4 | да / нет | Короткое название находки или рекомендации. | report.reports.payload.findings[] → title |
| `Finding.description` | `string` | `string` #5 | да / нет | Описание наблюдения на основании собранных данных. | report.reports.payload.findings[] → description |
| `Finding.severity` | `string; critical, high, medium, low, informational` | `string` #6 | да / нет | Серьёзность находки: critical, high, medium, low или informational. | report.reports.payload.findings[] → severity |
| `Finding.possible_consequence` | `string` | `string` #7 | да / нет | Возможное последствие находки; не утверждает уже произошедший инцидент. | report.reports.payload.findings[] → possible_consequence |
| `Finding.evidence_ids` | `array<string>` | `repeated string` #8 | да / нет | Идентификаторы доказательств из сохранённых MetricInput.evidence этого отчёта. | report.reports.payload.findings[] → evidence_ids |
| `Finding.vulnerabilities` | `array<Vulnerability>` | `repeated repopulse.common.v1.Vulnerability` #9 | да / нет | Известные уязвимости конкретных версий зависимостей; идентификаторы и серьёзность OSV. | report.reports.payload.findings[] → vulnerabilities |

## Recommendation

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Recommendation.id` | `string` | `string` #1 | да / нет | Идентификатор рекомендации, уникальный в пределах отчёта. | report.reports.payload.recommendations[] → id |
| `Recommendation.finding_id` | `string` | `string` #2 | да / нет | Обязательная связь с finding этого отчёта. | report.reports.payload.recommendations[] → finding_id |
| `Recommendation.title` | `string` | `string` #3 | да / нет | Короткое название находки или рекомендации. | report.reports.payload.recommendations[] → title |
| `Recommendation.action` | `string` | `string` #4 | да / нет | Конкретное предлагаемое действие из детерминированного каталога рекомендаций. | report.reports.payload.recommendations[] → action |
| `Recommendation.expected_benefit` | `string` | `string` #5 | да / нет | Ожидаемая польза предлагаемого действия. | report.reports.payload.recommendations[] → expected_benefit |
| `Recommendation.priority` | `string; high, middle, low` | `string` #6 | да / нет | Приоритет рекомендации: high, middle или low. | report.reports.payload.recommendations[] → priority |
| `Recommendation.complexity` | `string; low, medium, high` | `string` #7 | да / нет | Оценка сложности действия: low, medium или high. | report.reports.payload.recommendations[] → complexity |
| `Recommendation.variant_id` | `string; short, expanded` | `string` #8 | да / нет | Идентификатор допустимого текстового варианта из каталога рекомендаций. | report.reports.payload.recommendations[] → variant_id |
| `Recommendation.explanation` | `string` | `string` #9 | да / нет | Объяснение результата или рекомендации, связанное с сохранёнными основаниями. | report.reports.payload.recommendations[] → explanation |

## Explanation

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Explanation.mode` | `string; ai_catalog_selection, deterministic_fallback` | `string` #1 | да / нет | Источник выбранного текста: ai либо fallback. | report.reports.payload.explanation → mode |
| `Explanation.fallback_reason` | `string / null` | `optional string` #2 | да / да | ai_unavailable/ai_timeout/ai_invalid_response/ai_not_configured/no_findings либо null. | report.reports.payload.explanation → fallback_reason |
| `Explanation.model` | `string / null` | `optional string` #3 | да / да | Фактически вызванная модель; вызова не было=null. | report.reports.payload.explanation → model |
| `Explanation.catalog_version` | `string` | `string` #4 | да / нет | Версия каталога допустимых рекомендаций и текстовых вариантов. | report.reports.payload.explanation → catalog_version |
| `Explanation.schema_version` | `string` | `string` #5 | да / нет | Версия схемы ответа AI с допустимыми идентификаторами. | report.reports.payload.explanation → schema_version |
| `Explanation.prompt_version` | `string` | `string` #6 | да / нет | Версия инструкции для выбора объяснения. | report.reports.payload.explanation → prompt_version |

## Report

Единственный публичный Report DTO. Все пять категорий и 12 разных метрик обязательны. Без owner/session credentials.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Report.report_id` | `string (uuid)` | `string` #1 | да / нет | UUID сохранённого отчёта. | report.reports.id |
| `Report.analysis_id` | `string (uuid)` | `string` #2 | да / нет | UUID анализа; используется для чтения статуса и связи со снимком и отчётом. | report.reports.analysis_id |
| `Report.snapshot_id` | `string (uuid)` | `string` #3 | да / нет | UUID сохранённого снимка входных данных анализа. | report.reports.snapshot_id |
| `Report.repository` | `Repository` | `repopulse.common.v1.Repository` #4 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | report.reports.payload → repository |
| `Report.assessment_at` | `string (date-time)` | `google.protobuf.Timestamp` #5 | да / нет | Зафиксированный момент оценки t в UTC; не обновляется при чтении результата. | report.reports.payload → assessment_at |
| `Report.window` | `Window` | `repopulse.common.v1.Window` #6 | да / нет | Точные границы окна активности [t−140 дней, t). | report.reports.payload → window |
| `Report.completed_at` | `string (date-time)` | `google.protobuf.Timestamp` #7 | да / нет | Момент окончательного сохранения отчёта в UTC. | report.reports.completed_at |
| `Report.versions` | `Versions` | `repopulse.common.v1.Versions` #8 | да / нет | Версии схемы, методики, профилей, правил и параметров объяснения для воспроизводимости. | report.reports.payload → versions |
| `Report.overall_score` | `number / null` | `optional double` #9 | да / да | Итоговый Repo Health Score 0..100; null при невозможности расчёта. Ноль является оценкой. | report.reports.payload → overall_score |
| `Report.overall_reason` | `Reason / null` | `repopulse.common.v1.Reason` #10 | да / да | Причина отсутствия итоговой оценки; null при наличии числового результата. | report.reports.payload → overall_reason |
| `Report.summary` | `string` | `string` #11 | да / нет | Краткое резюме сохранённого отчёта. | report.reports.payload → summary |
| `Report.coverage` | `Coverage` | `Coverage` #12 | да / нет | Число рассчитанных, частичных, недоступных и неприменимых метрик. | report.reports.payload → coverage |
| `Report.categories` | `array<Category>` | `repeated Category` #13 | да / нет | Все пять категорий: activity, documentation, testing, security, ci_cd в установленном порядке. | report.reports.payload → categories |
| `Report.sources` | `array<SourceState>` | `repeated repopulse.common.v1.SourceState` #14 | да / нет | Состояния источников, время получения, объём выборки, причины ограничений и отпечатки. | report.reports.payload → sources |
| `Report.dependencies` | `array<Dependency>` | `repeated repopulse.common.v1.Dependency` #15 | да / нет | Прямые зависимости npm/Maven/Go; дедупликация по ecosystem/package/version, общий лимит 20. | report.reports.payload → dependencies |
| `Report.findings` | `array<Finding>` | `repeated Finding` #16 | да / нет | Детерминированные находки по метрикам; идентификаторы уникальны внутри отчёта. | report.reports.payload → findings |
| `Report.recommendations` | `array<Recommendation>` | `repeated Recommendation` #17 | да / нет | Действия из каталога, связанные с находками; AI не добавляет новых действий. | report.reports.payload → recommendations |
| `Report.explanation` | `Explanation` | `Explanation` #18 | да / нет | Сохранённый режим выбора текста, версии, модель и причина fallback. | report.reports.payload → explanation |

## ReportSummary

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ReportSummary.report_id` | `string (uuid)` | `string` #1 | да / нет | UUID сохранённого отчёта. | report.reports (проекция) → report_id |
| `ReportSummary.repository` | `Repository` | `repopulse.common.v1.Repository` #2 | да / нет | Идентичность GitHub-репозитория и зафиксированная ревизия; структура Repository. | report.reports (проекция) → repository |
| `ReportSummary.completed_at` | `string (date-time)` | `google.protobuf.Timestamp` #3 | да / нет | Момент окончательного сохранения отчёта в UTC. | report.reports (проекция) → completed_at |
| `ReportSummary.overall_score` | `number / null` | `optional double` #4 | да / да | Итоговый Repo Health Score 0..100; null при невозможности расчёта. Ноль является оценкой. | report.reports (проекция) → overall_score |
| `ReportSummary.partial` | `boolean` | `bool` #5 | да / нет | true, если отчёт содержит ограничения или неполные данные. | report.reports (проекция) → partial |

## HistoryPage

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `HistoryPage.items` | `array<ReportSummary>` | `repeated ReportSummary` #1 | да / нет | Сохранённые отчёты аккаунта текущей страницы истории; максимум 20 элементов. | report.history_views/report.history_view_items + report.reports (проекция) → items |
| `HistoryPage.next_cursor` | `string / null` | `optional string` #2 | да / да | Непрозрачный привязанный к аккаунту курсор фиксированного состава обхода. | report.history_views/report.history_view_items + report.reports (проекция) → next_cursor |

## ListReportsQuery

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ListReportsQuery.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `ListReportsQuery.cursor` | `string / null` | `optional string` #2 | да / да | Непрозрачный курсор фиксированного просмотра истории; null для первой страницы. | — |

## GetReportQuery

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `GetReportQuery.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `GetReportQuery.report_id` | `string (uuid)` | `string` #2 | да / нет | UUID сохранённого отчёта. | — |

## ReusableReportQuery

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ReusableReportQuery.context` | `VerifiedContext` | `repopulse.common.v1.VerifiedContext` #1 | да / нет | Проверенные Auth идентичности и сроки действия; клиент не задаёт этот объект. | — |
| `ReusableReportQuery.key` | `FreshnessKey` | `repopulse.common.v1.FreshnessKey` #2 | да / нет | Полный подтверждённый контекст поиска ранее сохранённого отчёта. | — |

## ReusableReportResult

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `ReusableReportResult.report` | `Report / null` | `Report` #1 | да / да | Подходящий сохранённый отчёт; отсутствие означает необходимость нового анализа. | — |

