# Компоненты RepoPulse

Источник: [архитектура №916](https://ai.nsu.ru/documents/916).

```mermaid
flowchart TD
    Frontend[Frontend] -->|HTTP JSON| ApiGateway[API Gateway]
    ApiGateway -->|gRPC| AuthService[Auth Service]
    ApiGateway -->|gRPC| ReportService[Report Service]
    ApiGateway -->|внутренний интерфейс| Analyzer[Analyzer]
    Analyzer --> Collection[Git / GitHub / OSV: сбор данных]
    Collection --> MetricAnalyzers[Пять анализаторов / 12 метрик]
    MetricAnalyzers --> Scoring[Score / каталог рекомендаций]
    Scoring --> Explanation[AI: выбор объяснения / fallback]
    Explanation --> ReportStorage[(PostgreSQL: снимок и отчёт)]
    AuthService --> AuthStorage[(PostgreSQL: auth)]
    ReportService --> ReportStorage
    Analyzer --> AnalyzerStorage[(PostgreSQL: analyzer / очередь)]
```

Auth владеет идентичностями и сессиями. Analyzer фиксирует владельца, момент
оценки, данные и результат; атомарно берёт задания из очереди. Report читает
неизменяемые отчёты и историю с проверкой доступа. PostgreSQL одна; схема report
хранит отчёты, analyzer — снимки и очередь, auth — идентичности.
Контракты транспорта находятся в сервисных пакетах `contracts/`.
