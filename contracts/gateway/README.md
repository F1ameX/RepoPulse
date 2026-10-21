# HTTP API Gateway

Источник: [OpenAPI 3.1](v1/openapi.json). JSON-модели подключены из сервисных
schemas.json. [Все поля JSON/gRPC](../../docs/field-map.md),
[полный flow данных](../../docs/architecture/data-flow.md).

| HTTP | Тело / параметры | Успешный ответ | Вызов компонента |
|---|---|---|---|
| POST /api/v1/auth/register | Credentials | 201 RegistrationResult | Auth.Register(Credentials) |
| POST /api/v1/auth/login | Credentials | 200 SessionInfo и account cookie | Auth.Login → IssuedSession |
| POST /api/v1/auth/guest-session | тела нет; cookie проверяет Gateway | 201 новая / 200 действующая SessionInfo | ResolveContext; при необходимости CreateGuestSession(Empty) |
| GET /api/v1/auth/session | cookie | 200 SessionContext | ResolveContext; ограничение текущего RPC описано в flow |
| POST /api/v1/auth/logout | тела нет; account cookie | 204 без тела, удаление account cookie | Auth.Logout(LogoutCommand) |
| POST /api/v1/repositories/validate | ValidateRepositoryRequest | 200 ValidatedRepository | Analyzer.ValidateRepository(ValidateRepositoryCommand) |
| POST /api/v1/analyses | CreateAnalysisRequest | 202 accepted / 200 reused: CreateAnalysisResponse | Analyzer.CreateAnalysis(CreateAnalysisCommand) |
| GET /api/v1/analyses/{analysis_id} | UUID в path | 200 AnalysisStatus, включая failed | Analyzer.GetAnalysis(GetAnalysisQuery) |
| GET /api/v1/reports/{report_id} | UUID в path | 200 Report | Report.GetReport(GetReportQuery) |
| GET /api/v1/reports | необязательный cursor в query | 200 HistoryPage | Report.ListReports(ListReportsQuery) |

## Тело и параметры

JSON передаётся с Content-Type application/json. Браузер не присылает Owner,
VerifiedContext, user_id или guest_session_id. Gateway получает контекст из Auth
и передаёт его отдельным полем внутренних команд. Неизвестные поля тела отклоняются.

В CreateAnalysisRequest два поля: repository_url и request_id (UUID отправки).
request_id сохраняется при потере ответа и повторной отправке. Параметры
analysis_id/report_id указывают уже сохранённые объекты; чтение не запускает анализ.
Неверный, отсутствующий и чужой ресурс дают одинаковый 404 после проверки сессии.

cursor — непрозрачная строка Report, привязанная к аккаунту и фиксированному
просмотру истории. Отсутствие cursor начинает новый просмотр. Размер страницы —
до 20, порядок completed_at DESC, report_id ASC; пользовательских фильтров нет.

## Cookie и заголовки

- repopulse_session — непрозрачный токен аккаунта; HttpOnly, Secure, SameSite=Lax,
  Path=/api/v1, срок 168 часов без продления.
- repopulse_guest — непрозрачный гостевой токен; те же атрибуты, браузерная cookie
  без Max-Age/Expires, серверный срок максимум 24 часа.
- IssuedSession.token остаётся между Auth и Gateway; в JSON браузеру попадает
  только SessionInfo, токен устанавливается через Set-Cookie.
- Запросы изменения состояния проверяют Origin/CSRF. Cookie не передаются в URL.
- X-Request-ID в ответе — диагностический trace_id, отдельный от request_id отправки.

## Представление данных и ошибки

JSON использует snake_case, строки UUID и RFC3339 UTC для времени. Обязательные
поля включаются при null; пустые списки передаются как []. Скалярное optional
поле gRPC переносится в nullable HTTP DTO: отсутствие → null, заданный ноль → 0,
заданная пустая строка → "". Поля-сообщения Timestamp/Reason также проверяются
на наличие при переносе. Валидация значений выполняется сервисами по схемам.

Во всех HTTP-ошибках тело ErrorEnvelope с error.code, message, trace_id и
field_errors. [Таблица HTTP/gRPC ошибок](../../docs/errors.md).
Нереализованные обработчики текущего каркаса возвращают 501 NOT_IMPLEMENTED;
таблица выше описывает целевое поведение контрактов.
