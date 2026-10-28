# RepoPulse
Health assessment service for GitHub repositories

## Docker

В корневом `docker-compose.yml` описаны API Gateway, Auth, Report, Analyzer,
frontend и общая PostgreSQL 17. Для backend используется один Dockerfile:
аргумент SERVICE выбирает точку входа. Frontend собирается в статику и обслуживается
Nginx; запросы `/api/` передаются в API Gateway.

Нужны только Docker с Compose. Из корня репозитория соберите образы и
запустите проект:

```sh
docker compose up -d --build
```

Контейнер `migrations` дождётся готовности PostgreSQL и применит миграции
в порядке **auth → analyzer → report**, с отдельной таблицей версий Goose
для каждого каталога. Auth и Analyzer запустятся только после его успешного
завершения. Устанавливать Goose на хост и применять миграции вручную не нужно.

| Сервис | Адрес с хоста |
|---|---|
| Frontend | http://localhost:3000 |
| API Gateway | http://localhost:8080; проверка: `/health` |
| Auth gRPC | localhost:9090 |
| PostgreSQL | localhost:5432, база/пользователь/пароль: repopulse |
| Report, Analyzer | Без внешних портов в текущей реализации |
| migrations | Без внешнего порта; завершается после применения миграций |

Реквизиты БД в Compose предназначены для локальной разработки. Данные сохраняются
в Docker volume `postgres_data`. При повторном запуске достаточно `docker compose up -d --build`;
контейнер миграций запускается снова, Goose применяет только новые миграции.

Состояние и логи:

```sh
docker compose ps -a
docker compose logs migrations
docker compose logs -f
curl http://localhost:8080/health
```

Остановка с сохранением данных БД: `docker compose down`.
Состояние `Exited (0)` у `migrations` означает успешное завершение. При ошибке
миграций проверьте `docker compose logs migrations`, исправьте причину и повторите
`docker compose up -d --build`.

В С1 Gateway подключён к Auth по gRPC: работают гостевая сессия, чтение её
контекста и валидация публичного GitHub URL. Frontend доступен с Origin
http://localhost:3000; POST проверяет его точное совпадение с PUBLIC_ORIGIN.
Для локального HTTP в Compose задано COOKIE_SECURE=false; при HTTPS используйте true.
Auth хранит хэши токенов в PostgreSQL. Analyzer исполняет очередь PostgreSQL
и безопасно читает Git; Report пока является каркасом. HTTP-запуск анализа
(#18478), коллекторы метрик и расчёт результата подключаются отдельно.
Сборка безопасных нормализованных входов (#18479) реализована в
`application/snapshot`; [подключение и границы](docs/architecture/analyzer-inputs.md).
Пока Processor не подключён, задание после чтения Git завершается failed,
без фиктивного отчёта. [Устройство исполнителя](docs/architecture/analyzer-execution.md).
Экран С1 выполняет только проверку URL и
доступности репозитория: не вызывает создание анализа, не создаёт фиктивный
analysis_id и не переходит к отчёту. Не реализованные маршруты следующих
спринтов пока возвращают 501 NOT_IMPLEMENTED.

Проверка гостевого сценария (из браузера frontend отправляет те же запросы):

```sh
curl -c /tmp/repopulse-cookies.txt -H 'Origin: http://localhost:3000' -X POST http://localhost:3000/api/v1/auth/guest-session
curl -b /tmp/repopulse-cookies.txt http://localhost:3000/api/v1/auth/session
curl -b /tmp/repopulse-cookies.txt -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' -d '{"repository_url":"https://github.com/pressly/goose"}' http://localhost:3000/api/v1/repositories/validate
```

Для запуска frontend вне Docker: `npm run dev` из project/frontend (порт 3000);
Vite передаёт `/api` в Gateway на localhost:8080. Для локального запуска Gateway
на HTTP задайте COOKIE_SECURE=false.

## Документация

- [Backend на Go: запуск, конфигурация и проверки](project/backend/README.md)
- [Пошаговый запуск Analyzer с PostgreSQL](project/backend/README.md#запуск-analyzer-с-postgresql)

- [Контракты MVP по сервисам](contracts/README.md)
- [Поля JSON и gRPC](docs/field-map.md)
- [Flow данных](docs/architecture/data-flow.md)
- [Техническая проверка Git](docs/git-check.md)
