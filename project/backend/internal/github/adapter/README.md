# Адаптеры GitHub API Service

Сбор и нормализация platform-specific данных GitHub и сохранение снимков.

Работает только `in/http`: служебный `GET /health`, логирование и request ID.
Запланированные доменные адаптеры:

- `in/grpc` — gRPC-методы сервиса;
- `in/kafka` — Kafka consumers команд или событий;
- `out/github` — клиент github;
- `out/kafka` — публикация команд или событий;
- `out/repository` — хранилище сервиса;

Доменные адаптеры зарезервированы для последующей реализации. gRPC-сервер,
Kafka consumers/producers и внешние подключения сейчас не запускаются.
