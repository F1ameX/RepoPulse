# RepoPulse
Health assessment service for GitHub repositories

## Docker — Спринт 1

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

Compose запускает текущую реализацию: Auth обслуживает gRPC, Analyzer открывает
хранилище, Report пока является каркасом. Подключение Auth к HTTP Gateway,
исполнение очереди и остальные бизнес-обработчики развиваются в отдельных задачах.

## Документация

- [Backend на Go: запуск, конфигурация и проверки](project/backend/README.md)
- [Пошаговый запуск Analyzer с PostgreSQL](project/backend/README.md#запуск-analyzer-с-postgresql)

- [Контракты MVP по сервисам](contracts/README.md)
- [Поля JSON и gRPC](docs/field-map.md)
- [Flow данных](docs/architecture/data-flow.md)
- [Техническая проверка Git](docs/git-check.md)
