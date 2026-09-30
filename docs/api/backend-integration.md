# Контракт backend API для согласования

## Локальный запуск

Из корня репозитория:

```sh
cd project/backend
go run ./cmd/api
```

Проверка backend во втором терминале (в PowerShell используйте `curl.exe`):

```sh
curl http://127.0.0.1:8080/health
```

Ожидаемый ответ: `{"status":"ok"}`. Адрес backend задаётся через `HTTP_ADDR`.
Согласование клиентских маршрутов и настройка браузерного подключения
(reverse proxy или CORS) относятся к отдельной задаче интеграции.

## Что можно проверить сейчас

| Запрос | Результат |
| --- | --- |
| `GET /health` | `200`, `{"status":"ok"}` |
| `POST /api/v1/analyses` | `501`, `error.code = NOT_IMPLEMENTED` |
| `GET /api/v1/analyses/A123` | `501`, `error.code = NOT_IMPLEMENTED` |
| `GET /api/v1/reports/R123` | `501`, `error.code = NOT_IMPLEMENTED` |

`501` означает, что маршрут зарезервирован, а доменный сервис ещё не подключён.
Эти ответы не следует интерпретировать как успешный запуск или готовый отчёт.

Для ошибок маршрутизации и заглушек возвращается:

```json
{
  "error": {
    "code": "NOT_IMPLEMENTED",
    "message": "Analysis and report services are not implemented yet",
    "request_id": "generated-request-id"
  }
}
```

`request_id` совпадает с заголовком `X-Request-ID` и записью в логах backend.
Машиночитаемый контракт текущего поведения:
[OpenAPI](../../project/backend/docs/openapi.json).

## Следующий этап: успешные ответы

Ниже — план по архитектурному документу, **ещё не реализованный** в каркасе.
Это предложение контракта со стороны backend для последующего согласования.

1. `POST /api/v1/analyses` с JSON:

   ```json
   { "repository_url": "https://github.com/example/project" }
   ```

   Новый анализ: `{"status":"created","analysis_id":"A123"}`.
   Актуальный отчёт: `{"status":"report_exists","report_id":"R123"}`.

2. При новом анализе выполнять polling `GET /api/v1/analyses/A123`.
   Статусы: `queued`, `running`, `completed`, `failed`. Промежуточный ответ:

   ```json
   {
     "analysis_id": "A123",
     "status": "running",
     "current_stage": "ANALYZING",
     "stages": [
       { "name": "PREPARING", "status": "completed" },
       { "name": "COLLECTING", "status": "completed" },
       { "name": "ANALYZING", "status": "running" },
       { "name": "SCORING", "status": "pending" },
       { "name": "RECOMMENDING", "status": "pending" },
       { "name": "REPORTING", "status": "pending" }
     ]
   }
   ```

3. Завершение: `{"analysis_id":"A123","status":"completed","report_id":"R123"}`.
   После этого запросить `GET /api/v1/reports/R123`.

При `report_exists` из первого ответа перейти сразу к получению отчёта.
`analysis_id` и `report_id` — разные идентификаторы. Точные успешные HTTP-коды,
ошибки анализа, контракт Report и авторизация должны быть добавлены в OpenAPI
вместе с реальной реализацией сервисов. Старые `/repositories`, `/analysis/{id}`,
`/report/{id}` не поддерживаются.
