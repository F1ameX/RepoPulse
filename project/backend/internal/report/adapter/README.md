# Адаптеры Report Service

Генерация, хранение и выдача immutable отчётов с проверкой доступа.

Работает только `in/http`: служебный `GET /health`, логирование и request ID.
Запланированные доменные адаптеры:

- `in/grpc` — gRPC-методы сервиса;
- `in/kafka` — Kafka consumers команд или событий;
- `out/github` — клиент github;
- `out/analysis` — клиент analysis;
- `out/scoring` — клиент scoring;
- `out/recommendation` — клиент recommendation;
- `out/kafka` — публикация команд или событий;
- `out/repository` — хранилище сервиса;
- `out/redis` — кэш;

Доменные адаптеры зарезервированы для последующей реализации. gRPC-сервер,
Kafka consumers/producers и внешние подключения сейчас не запускаются.
