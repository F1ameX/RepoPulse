# HTTP-типы данных RepoPulse MVP

Структура HTTP-запросов и ответов описана в [OpenAPI](../../project/backend/api/openapi/openapi.json). Пример итогового отчёта: [report.json](../../project/backend/api/examples/report.json).

Для страницы отчёта используются `Report`, `Repository`, `AnalysisWindow`, `CategoryScore`, `Finding`, `Recommendation`, `Completeness` и `SourceCollectionState`. Для истории — `ReportList` и `ReportSummary`; для polling — `AnalysisStatus`.

HTTP JSON использует `snake_case`. В отчёте остаются оценка, краткие объяснения, findings, рекомендации и полнота данных. Недоступная оценка представлена `null`, а не нулём. Статусы данных: `complete`, `partial`, `unavailable`, `not_applicable`. Отдельного Evidence и оценки структуры репозитория нет.

Модель не включает проверки зависимостей, данные package registries, coverage или результат запуска тестов. Существующий код frontend ещё использует прежние DTO и требует адаптации к этому контракту при реализации интеграции.
