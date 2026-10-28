# Ошибки HTTP / gRPC

Во всех HTTP-ошибках одно тело ErrorEnvelope; внутри gRPC используется тот же
ApiError в бинарной trailing metadata `repopulse-error-bin` плюс стандартный
gRPC status. Значения внутренних исключений, stderr Git, ответы провайдера,
пароли, токены и содержимое секретов не передаются. trace_id — диагностика,
request_id отправки остаётся у клиента и в submissions. Внутренний адаптер
преобразует gRPC status + ApiError в HTTP-код по этой таблице.

| Случай / code | HTTP | gRPC | Сообщение / правило |
|---|---|---|---|
| URL_REQUIRED | 400 | INVALID_ARGUMENT | Укажите URL репозитория |
| URL_TOO_LONG | 400 | INVALID_ARGUMENT | URL должен содержать не более 2048 символов |
| INVALID_REPOSITORY_URL | 400 | INVALID_ARGUMENT | Укажите ссылку вида https://github.com/owner/repository |
| UNSUPPORTED_PLATFORM | 400 | INVALID_ARGUMENT | Поддерживаются только репозитории GitHub |
| VALIDATION_FAILED | 400 | INVALID_ARGUMENT | Первая ошибка каждого поля в field_errors, порядок FR-01/27 |
| INVALID_CREDENTIALS | 401 | UNAUTHENTICATED | Неверный логин или пароль; неизвестный логин не раскрывается |
| LOGIN_TAKEN | 409 | ALREADY_EXISTS | Этот логин уже занят |
| SESSION_REQUIRED / SESSION_EXPIRED | 401 | UNAUTHENTICATED | Нужен действующий контекст; истечение не переключает прежнюю операцию на гостя |
| LOGIN_REQUIRED | 401 | UNAUTHENTICATED | Войдите, чтобы открыть историю отчётов; до проверки существования конкретного объекта |
| CSRF_REJECTED | 403 | PERMISSION_DENIED | Запрос не прошёл проверку источника |
| REPOSITORY_NOT_FOUND_OR_PRIVATE | 404 | NOT_FOUND | Публичный репозиторий недоступен; private/404/подтверждённый запрет одинаковы |
| REPOSITORY_CHECK_FAILED | 503 | UNAVAILABLE | Не удалось проверить доступность репозитория. Повторите попытку позже |
| UPSTREAM_RATE_LIMITED | 503 | RESOURCE_EXHAUSTED | Лимит источника; 403 rate-limit не превращается в «репозитория нет» |
| UPSTREAM_TIMEOUT | 504 | DEADLINE_EXCEEDED | Источник не ответил вовремя |
| SUBMISSION_CHANGED | 409 | ALREADY_EXISTS | Запрос изменён. Отправьте его заново |
| SUBMISSION_PENDING | 409 | ABORTED | Исходная отправка ещё проверяется. Безопасный повтор с тем же request_id; новый job не создаётся |
| ANALYSIS_NOT_FOUND | 404 | NOT_FOUND | Анализ недоступен; неверный ID/чужой/отсутствующий объект одинаковы |
| REPORT_NOT_FOUND | 404 | NOT_FOUND | Отчёт недоступен; неверный ID/чужой/отсутствующий объект одинаковы |
| AUTH_UNAVAILABLE | 503 | UNAVAILABLE / DEADLINE_EXCEEDED | Auth недоступен или не ответил за AUTH_REQUEST_TIMEOUT; токены/адреса/внутренние сообщения не раскрываются |
| INTERNAL_ERROR | 500 | INTERNAL | Не удалось выполнить операцию |
| NOT_FOUND | 404 | NOT_FOUND | Неизвестный HTTP-маршрут каркаса, без метаданных ресурса |
| METHOD_NOT_ALLOWED | 405 | UNIMPLEMENTED | Неподдерживаемый HTTP-метод, Allow header |
| NOT_IMPLEMENTED | 501 | UNIMPLEMENTED | Только существующий каркас; не подтверждает готовность функции |

ANALYSIS_FAILED, BUDGET_EXCEEDED, REPORT_SAVE_FAILED — коды терминального
AnalysisStatus.error. Чтение сохранённого failed-статуса само возвращает HTTP 200.
Сбой чтения статуса/отчёта возвращает транспортную ошибку и не меняет анализ.
Сбой независимого источника может дать partial-отчёт вместо failed;
у метрик сохраняются reason.code/message/cannot_conclude.

Ошибка формата URL до принятия анализа не создаёт job. Нормализация и порядок
проверок повторяются на сервере по документу №888. В частности, запрещены port,
userinfo, query/fragment (даже пустые), percent-encoding, дополнительные сегменты,
backslash, внутренние пробелы и не-ASCII; только HTTPS github.com, owner/repository,
один конечный slash и удаление одного суффикса .git. Метаданные GitHub, а не один
HTTP-статус или HTML-страница, подтверждают публичность и перенос с тем же GitHub ID.

## Ошибки полей login/password (FR-27)

VALIDATION_FAILED содержит в field_errors первую ошибку каждого поля: login, затем
password. Проверки поля идут в порядке пустота → длина → символы → только пробелы;
проверка занятости и учётных данных выполняется только после прохождения обоих полей.
Сообщение не повторяет введённый пароль.

| field | code | message |
|---|---|---|
| login | REQUIRED | Укажите логин |
| login | INVALID_LENGTH | Логин должен содержать от 3 до 32 символов |
| login | INVALID_CHARACTERS | Логин: первая латинская буква, далее латинские буквы, цифры или _ |
| password | REQUIRED | Укажите пароль |
| password | INVALID_LENGTH | Пароль должен содержать от 15 до 128 символов |
| password | INVALID_CHARACTERS | Пароль содержит недопустимые символы |
| password | WHITESPACE_ONLY | Пароль не может состоять только из пробелов |

Отказ входа и сбой входа различаются: неизвестный login и неверный пароль дают
INVALID_CREDENTIALS (401), а сбой БД, хэша пароля или Auth — INTERNAL_ERROR (500)
либо AUTH_UNAVAILABLE (503). Временный сбой не объявляется неверным паролем.
Выход идемпотентен (204); если Auth не подтвердил отзыв сессии, возвращается
ошибка, а account cookie не удаляется.
