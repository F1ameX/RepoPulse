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

