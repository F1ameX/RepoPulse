# Миграции по сервисам

| Каталог / схема | Таблицы |
|---|---|
| auth | principals, sessions |
| analyzer | submissions, analyses, snapshots, snapshot_dependencies, snapshot_vulnerabilities |
| report | reports, history_views, history_view_items |
| gateway | [постоянных таблиц нет](gateway/README.md) |

В каждом сервисе одна миграция `00001_foundation.sql` с секциями Goose Up/Down.
Up создаёт схему, таблицы и индексы; Down удаляет их в обратном порядке.
Оставлены первичные/внешние ключи, NOT NULL и уникальность записей.

Для общей PostgreSQL порядок применения: **auth → analyzer → report**.
Порядок отката: **report → analyzer → auth**.

Валидация данных, проверка владельца, переходы статусов, сроки сессий,
неизменяемость результатов и транзакционное завершение анализа реализуются
в сервисах.

Задача хранения использует существующие foundation-таблицы; изменять уже
применённые `00001_foundation.sql` для её запуска не требуется.
При использовании Goose CLI задавайте отдельную таблицу версий для каждого
каталога: номера миграций в трёх каталогах независимы. Из `project/backend`:

```sh
goose -dir migrations/auth -table goose_auth_version postgres "<DSN>" up
goose -dir migrations/analyzer -table goose_analyzer_version postgres "<DSN>" up
goose -dir migrations/report -table goose_report_version postgres "<DSN>" up
```

Для Down выполняйте команды в обратном порядке с теми же таблицами версий.
Приложение не применяет миграции само. PostgreSQL integration-тест адаптера
Analyzer проверяет SQL секций Up/Down на отдельной временной БД; он не заменяет
учёт версий Goose в окружении разработчика.

