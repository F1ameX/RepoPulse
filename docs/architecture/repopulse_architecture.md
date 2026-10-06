# Архитектура RepoPulse

# Назначение системы

**RepoPulse** — распределённая система для автоматизированной оценки технического состояния публичного GitHub-репозитория.

Пользователь указывает URL публичного GitHub-репозитория. Система:

1. проверяет, существует ли уже актуальный отчёт;
2. при необходимости собирает данные из GitHub API;
3. клонирует Git-репозиторий в изолированную sandbox-среду;
4. выполняет технический анализ репозитория;
5. рассчитывает Repo Health Score;
6. формирует рекомендации;
7. создаёт и сохраняет итоговый отчёт;
8. предоставляет авторизованному пользователю историю его отчётов и скачивание PDF.

## Режимы доступа

| Действие | Гость | Авторизованный пользователь |
|---|---|---|
| Запустить анализ публичного репозитория по URL | Да | Да |
| Читать статус своего анализа | Да | Да |
| Смотреть свой готовый отчёт | Да, в рамках гостевой сессии | Да |
| Читать историю своих отчётов, включая историю по репозиторию | Нет | Да |
| Скачать свой отчёт в PDF | Нет | Да |

Гость работает без регистрации аккаунта. Для доступа к своему анализу и отчёту он получает гостевую сессию. Гостевые отчёты сохраняются на сервере для получения результата, но история гостю не предоставляется.

Вход или регистрация не меняют владельца гостевых анализов и отчётов: они не переносятся в аккаунт, не включаются в его историю и не становятся доступны для скачивания PDF. Анализы, запущенные после входа, принадлежат авторизованному пользователю.

Основной pipeline анализа:

```text
PREPARING
    ↓
COLLECTING
    ↓
ANALYZING
    ↓
SCORING
    ↓
RECOMMENDING
    ↓
REPORTING
    ↓
COMPLETED
```

При критической ошибке анализ переходит в состояние:

```text
FAILED
```

# Общая архитектура

## Контекстная диаграмма — C4 Level 1

Диаграмма показывает RepoPulse как единую систему, две роли пользователей и взаимодействие с GitHub, SonarQube Server и AI/LLM Provider. Внутренние сервисы, Kafka, базы данных, кэш и инфраструктура наблюдаемости раскрываются на уровне контейнеров.

![RepoPulse — C4 Level 1: System Context Diagram](diagrams/RepoPulse_C4_Level1.png)

[Исходник PlantUML](diagrams/context.puml).

## Диаграмма контейнеров — C4 Level 2

Основные сервисы RepoPulse:

- API Service / BFF;
- Auth Service;
- Analysis Orchestrator;
- GitHub API Service;
- Repository Sandbox Service;
- Analysis Service;
- Scoring Service;
- Recommendation Service;
- Report Service.

Инфраструктурные компоненты:

- Kafka;
- PostgreSQL;
- Redis;
- SonarQube Server;
- AI/LLM Provider;
- Elasticsearch;
- Logstash;
- Kibana;
- Prometheus;
- Grafana.

Общая схема:

```text
Frontend
   │
   │ HTTP/JSON
   ▼
API Service / BFF
   │
   ├── gRPC → Auth Service
   ├── gRPC → Analysis Orchestrator
   └── gRPC → Report Service

Analysis Orchestrator
   │
   ├── gRPC → GitHub API Service
   ├── gRPC → Report Service
   │
   └── Kafka
        │
        ├── GitHub API Service
        ├── Repository Sandbox Service
        ├── Analysis Service
        ├── Scoring Service
        ├── Recommendation Service
        └── Report Service

Repository Sandbox Service
   │
   └── gVisor sandbox
          ├── git clone
          ├── local repository
          ├── RepoPulse collector: Git/files
          └── SonarScanner
                 │
                 ▼
            SonarQube Server

Scoring Service ────────→ AI/LLM Provider
Recommendation Service ─→ AI/LLM Provider
```

Диаграмма контейнеров C4 Level 2: [PNG](diagrams/RepoPulse_C4_Level2.png), [PlantUML](diagrams/architecture.puml).

Хранение и инфраструктура для MVP:

```text
1 PostgreSQL container
├── auth_db
├── orchestrator_db
├── github_db
├── analysis_db
├── scoring_db
├── recommendation_db
└── report_db

1 Redis
1 Kafka
1 SonarQube Server

1 Elasticsearch
1 Logstash
1 Kibana

1 Prometheus
1 Grafana
```

Каждый сервис имеет собственную database, собственного DB user и не читает таблицы других сервисов напрямую.

Redis используется как cache, а не как source of truth.

Kafka используется как transport для тяжёлых асинхронных операций и не является source of truth для состояния анализа.

---

# 1. API Service / BFF

## Ответственность

API Service является единой точкой входа frontend в backend RepoPulse.

Он:

- предоставляет внешний HTTP API;
- выполняет базовую валидацию входных данных;
- проверяет access JWT пользователя или подписанную cookie гостевой сессии;
- формирует проверенный контекст `Requester`: `user_id` либо `guest_session_id`;
- разрешает историю и PDF только авторизованному пользователю;
- преобразует внешние DTO во внутренние gRPC-запросы;
- маршрутизирует команды и запросы во внутренние сервисы;
- выполняет error mapping;
- добавляет `request_id`;
- не содержит бизнес-логику анализа;
- не работает напрямую с PostgreSQL;
- не является владельцем доменных данных.

## Связи

```text
Frontend → HTTP → API Service

API Service → gRPC → Auth Service
API Service → gRPC → Analysis Orchestrator
API Service → gRPC → Report Service
```

## Контекст инициатора и проверка доступа

Общий внутренний контекст для запуска анализа, чтения статуса, чтения отчёта и поиска reusable report:

Тип определён в [common.proto](../../project/backend/api/proto/repopulse/common/v1/common.proto). Получатель проверяет наличие выбранной идентичности и непустой ID: `oneof` запрещает одновременное заполнение двух полей, но допускает отсутствие обоих.

```protobuf
message Requester {
  oneof identity {
    string user_id = 1;
    string guest_session_id = 2;
  }
}
```

API Service получает идентификаторы только из проверенных credentials и передаёт `Requester` по доверенному внутреннему gRPC-вызову. Frontend не может назначить владельца через JSON-поля `user_id` или `guest_session_id`.

При наличии `Authorization: Bearer ...` используется пользовательский access JWT. Некорректный или истёкший access JWT приводит к `401`, а не к переключению запроса в гостевой режим. Без access JWT для гостевых операций проверяется cookie `repopulse_guest`.

Для истории и PDF обязательно наличие действующего пользовательского access JWT: одной гостевой cookie недостаточно. Проверка владельца конкретного анализа выполняется в Orchestrator, а конкретного отчёта — в Report Service. Чужой ресурс возвращает `404`, включая запросы с известным `analysis_id` или `report_id`.

## OpenAPI-контракты

Полная спецификация: [openapi.json](../../project/backend/api/openapi/openapi.json). Общий указатель gRPC, Kafka и HTTP-контрактов: [mvp-contracts.md](mvp-contracts.md). HTTP использует `snake_case`; API явно преобразует внутренние protobuf-модели в DTO. Создание анализа возвращает `202`, существующий отчёт — `200`, регистрация и создание гостевой сессии — `201`.

### POST `/api/v1/auth/guest-session`

Создаёт гостевую сессию без логина и пароля. Frontend вызывает маршрут перед первым гостевым анализом, если действующей гостевой cookie ещё нет.

API Service вызывает `AuthService.CreateGuestSession()` и устанавливает cookie `repopulse_guest` с атрибутами `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/api/v1`. Токен не возвращается в JSON. Срок cookie совпадает со сроком гостевого токена.

Response:

```json
{
  "expires_at": "2026-10-02T10:00:00Z"
}
```

Значение времени приведено как пример; срок гостевой сессии задаётся конфигурацией. Для запросов, изменяющих состояние и использующих cookie, API применяет CSRF-защиту с проверкой Origin.

### POST `/api/v1/auth/register`

Request:

```json
{
  "login": "user",
  "password": "password"
}
```

Response:

```json
{
  "user_id": "uuid"
}
```

### POST `/api/v1/auth/login`

Request:

```json
{
  "login": "user",
  "password": "password"
}
```

Response:

```json
{
  "access_token": "...",
  "refresh_token": "..."
}
```

### POST `/api/v1/auth/refresh`

Request:

```json
{
  "refresh_token": "..."
}
```

Response:

```json
{
  "access_token": "...",
  "refresh_token": "..."
}
```

### POST `/api/v1/analyses`

Доступен гостю с действующей гостевой cookie и авторизованному пользователю с access JWT. Создаёт новый анализ или возвращает существующий актуальный Report того же владельца.

API Service добавляет проверенный `Requester` к внутреннему `StartAnalysisRequest`. Тело запроса содержит только URL репозитория.

Request:

```json
{
  "repository_url": "https://github.com/example/project"
}
```

Response при создании анализа:

```json
{
  "status": "created",
  "analysis_id": "A123"
}
```

Response при наличии актуального отчёта:

```json
{
  "status": "report_exists",
  "report_id": "R123"
}
```

### GET `/api/v1/analyses/{analysis_id}`

Доступен обоим режимам. Orchestrator сверяет `Requester` с владельцем `AnalysisRun` перед выдачей статуса, включая ответ из Redis.

Response:

```json
{
  "analysis_id": "A123",
  "status": "running",
  "current_stage": "ANALYZING",
  "stages": [
    {
      "name": "PREPARING",
      "status": "completed"
    },
    {
      "name": "COLLECTING",
      "status": "completed"
    },
    {
      "name": "ANALYZING",
      "status": "running"
    },
    {
      "name": "SCORING",
      "status": "pending"
    },
    {
      "name": "RECOMMENDING",
      "status": "pending"
    },
    {
      "name": "REPORTING",
      "status": "pending"
    }
  ]
}
```

Необязательное поле `stages[].progress_percent` (`0..100`) возвращается только там, где backend действительно может подтвердить процент прогресса.

### GET `/api/v1/reports/{report_id}`

Возвращает компактный [Report DTO](../../project/backend/api/openapi/openapi.json) по [примеру](../../project/backend/api/examples/report.json), принадлежащий текущему `Requester`. Для гостя проверяется совпадение `guest_session_id`, для пользователя — `user_id`. Гостевой доступ действует до истечения гостевой сессии.

### GET `/api/v1/reports/{report_id}/pdf`

Доступен только авторизованному пользователю. API Service вызывает `ReportService.GetReportPdf(user_id, report_id)`.

Report Service проверяет владельца и формирует PDF из сохранённого immutable Report по запросу. API Service передаёт полученный поток в HTTP-ответ:

```http
Content-Type: application/pdf
Content-Disposition: attachment; filename="repopulse-report-R123.pdf"
Cache-Control: private, no-store
```

Гостевой запрос получает `401` с кодом `login_required`. Авторизованный запрос к чужому или гостевому отчёту получает `404`.

### GET `/api/v1/reports`

Доступен только с пользовательским access JWT. Возвращает историю отчётов текущего `user_id`; гостевые отчёты в неё не включаются.

Оба маршрута истории возвращают `reports`, `total_count`, `limit`, `offset`. `limit` по умолчанию `20`, допустимый диапазон `1..100`; `offset` начинается с нуля. Фильтр `repository` имеет формат `owner/name`. Порядок: `generated_at DESC, report_id DESC`.

Возможные query-параметры:

```text
limit
offset
repository
```

### GET `/api/v1/repositories/{owner}/{name}/reports`

Доступен только с пользовательским access JWT. Возвращает историю отчётов текущего `user_id` для конкретного repository. Оба маршрута истории возвращают гостю `401` с кодом `login_required`.

---

# 2. Auth Service

## Ответственность

Auth Service отвечает за идентификацию пользователей.

Для MVP используется авторизация по логину и паролю.

Auth Service:

- регистрирует пользователей;
- хранит пользователей;
- хранит password hash;
- проверяет логин и пароль;
- выдаёт Access Token;
- выдаёт Refresh Token;
- обновляет Access Token;
- завершает пользовательские сессии;
- создаёт гостевые сессии и выдаёт подписанный гостевой токен для cookie без создания пользователя.

API Service проверяет JWT на каждом защищённом запросе.

Проверка доступа к конкретному ресурсу выполняется доменным сервисом-владельцем ресурса.

```text
API Service:
"Кто инициатор: пользователь или гостевая сессия?"

Analysis Orchestrator:
"Принадлежит ли analysis_id этому инициатору?"

Report Service:
"Принадлежит ли report_id этому инициатору и доступна ли ему запрошенная операция?"
```

## Гостевая сессия

`CreateGuestSession` создаёт случайный `guest_session_id` и сохраняет сессию в `auth_db`. Auth Service подписывает отдельный гостевой JWT с `principal_type=guest`, `sub=guest_session_id`, `iss`, `aud` и `exp`. API Service проверяет подпись, назначение, тип и срок токена локально. Пользовательский access JWT имеет `principal_type=user`; гостевой токен не принимается как пользовательский.

Гостевой токен передаётся через `HttpOnly` cookie. Для гостя не создаются запись в `users` и refresh-сессия. По истечении сессии старые анализы и отчёты перестают быть доступны гостю; новая сессия имеет другой идентификатор. Длительность задаётся конфигурацией и должна позволять завершить анализ и просмотреть результат.

Гостевая сессия не преобразуется в пользовательскую при входе или регистрации. Auth Service не переносит гостевые отчёты и не меняет их владельца.

## gRPC-контракт

Полный контракт с сообщениями запросов и ответов: [auth.proto](../../project/backend/api/proto/repopulse/auth/v1/auth.proto).

```protobuf
service AuthService {
  rpc CreateGuestSession(CreateGuestSessionRequest)
      returns (CreateGuestSessionResponse);

  rpc Register(RegisterRequest)
      returns (RegisterResponse);

  rpc Login(LoginRequest)
      returns (LoginResponse);

  rpc Refresh(RefreshRequest)
      returns (RefreshResponse);
}
```

Логически:

```text
CreateGuestSessionResponse {
    guest_token
    expires_at
}

Register {
    login
    password
}

Login {
    login
    password
}

Refresh {
    refresh_token
}
```

`RegisterResponse` содержит `user_id`. `LoginResponse` и `RefreshResponse` содержат `access_token` и `refresh_token`, как в HTTP-контрактах API Service. `CreateGuestSessionRequest` не содержит параметров; срок сессии определяет сервер. `expires_at` в ответе обязателен и совпадает с `exp` гостевого JWT.

Ошибки передаются через стандартный gRPC status: `INVALID_ARGUMENT` для некорректных параметров, `ALREADY_EXISTS` для занятого login, `UNAUTHENTICATED` для неверных учётных данных или недействительной refresh-сессии. Ошибка входа не различает неизвестный login и неверный пароль. См. [gRPC status codes](https://grpc.io/docs/guides/status-codes/).

## Хранилище

Database:

```text
auth_db
```

Таблица `users`:

```text
users
├── id
├── login
├── password_hash
├── created_at
└── updated_at
```

Таблица `refresh_sessions`:

```text
refresh_sessions
├── id
├── user_id
├── token_hash
├── expires_at
├── revoked_at
└── created_at
```

Таблица `guest_sessions`:

```text
guest_sessions
├── id
├── expires_at
└── created_at
```

Source of truth для данных сессий — `auth_db`; Redis для выдачи и локальной проверки гостевого JWT не требуется. Истечение гостевого доступа проверяется по `exp` подписанного токена.

---

# 3. Analysis Orchestrator

## Ответственность

Analysis Orchestrator является владельцем workflow анализа.

Он:

- создаёт `AnalysisRun` с владельцем — пользователем или гостевой сессией;
- хранит текущее состояние анализа;
- хранит текущий stage;
- инициирует следующие этапы;
- ставит тяжёлые задачи через Kafka;
- принимает события о завершении этапов;
- отслеживает progress;
- выполняет локальный retry собственных синхронных gRPC-вызовов;
- обрабатывает timeout;
- хранит ошибки;
- обеспечивает idempotency;
- выполняет preflight-проверку;
- решает, нужен ли новый анализ;
- предоставляет статус анализа API Service после проверки владельца;
- передаёт сохранённого владельца в команду `GenerateReport`.

## Состояния анализа

Основной pipeline:

```text
PREPARING
    ↓
COLLECTING
    ↓
ANALYZING
    ↓
SCORING
    ↓
RECOMMENDING
    ↓
REPORTING
    ↓
COMPLETED
```

Внешний статус:

```text
QUEUED
RUNNING
COMPLETED
FAILED
```

## PREPARING

На этапе `PREPARING` Orchestrator выполняет синхронные gRPC-вызовы:

```text
Orchestrator
   ├── gRPC → GitHub API Service
   │          ResolveRepositoryState()
   │
   └── gRPC → Report Service
              FindReusableReport(requester, repository, updated_at)
```

Для MVP актуальность определяется через:

```text
GET /repos/{owner}/{repo}
→ repository.updated_at
```

Если `updated_at` совпадает со значением в последнем отчёте того же владельца, новый анализ не запускается. `FindReusableReport` получает проверенный `Requester`; поиск не выдаёт отчёты других пользователей или других гостевых сессий.

Если требуется новый анализ, `ResolveRepositoryState` также получает основную ветку и её полный head SHA. Orchestrator фиксирует `repository_revision`, `analysis_window` и `collector_profile_version` в AnalysisRun до отправки команд `COLLECTING`. Обе команды получают одинаковые SHA и окно, поэтому GitHub API Service и Repository Sandbox Service собирают данные параллельно без зависимости друг от друга. Orchestrator ожидает `GitHubDataCollected` и `RepositoryDataCollected`, сохраняет `github_snapshot_id`, `sandbox_id` и `sandbox_collection_id` и проверяет SHA результата sandbox. Только после обоих событий он отправляет `AnalyzeRepository` в Analysis Service. Событие sandbox означает готовность локальных фактов и загрузку отчёта SonarScanner; серверную обработку SonarQube ожидает Analysis Service.

## gRPC-контракт

Полный контракт с сообщениями запросов и ответов: [orchestrator.proto](../../project/backend/api/proto/repopulse/orchestrator/v1/orchestrator.proto).

```protobuf
service AnalysisOrchestrator {
  rpc StartAnalysis(StartAnalysisRequest)
      returns (StartAnalysisResponse);

  rpc GetAnalysisStatus(GetAnalysisStatusRequest)
      returns (GetAnalysisStatusResponse);
}
```

Логический запрос:

```text
StartAnalysis {
    requester
    repository_url
}

GetAnalysisStatus {
    requester
    analysis_id
}
```

`Requester` содержит ровно одну идентичность. Orchestrator сохраняет её при создании анализа и проверяет при чтении статуса. Наличие одного `analysis_id` не предоставляет доступ.

Результат:

```text
AnalysisCreated {
    analysis_id
}

или

ExistingReport {
    report_id
}
```

## Kafka-команды

Orchestrator является producer команд:

```text
CollectGitHubData
CollectRepositoryData
AnalyzeRepository
CalculateScore
GenerateRecommendations
GenerateReport
```

Kafka используется как control plane: через неё передаются команды, события и идентификаторы результатов, но не полные доменные данные.

Основной принцип:

```text
Kafka:
analysis_id
snapshot_id
result_id
```

Сами данные сервисы получают у сервиса-владельца через gRPC.

Kafka key:

```text
analysis_id
```

Общий envelope задаётся [messages.schema.json](../../project/backend/api/kafka/messages.schema.json). Топики и допустимые типы сообщений перечислены в [topics.json](../../project/backend/api/kafka/topics.json). Ниже показана форма envelope; payload выбирается по `type`. Примеры в разделах сервисов сокращены; полные сообщения находятся в [kafka-messages.json](../../project/backend/api/examples/kafka-messages.json).

```json
{
  "message_id": "uuid",
  "analysis_id": "uuid",
  "type": "AnalyzeRepository",
  "created_at": "2026-09-30T10:00:00Z",
  "payload": {}
}
```

## Kafka events

Orchestrator является основным consumer событий:

```text
GitHubDataCollectionStarted
GitHubDataCollected
GitHubDataCollectionFailed

RepositoryDataCollectionStarted
RepositoryDataCollected
RepositoryDataCollectionFailed

AnalysisStarted
AnalysisCompleted
AnalysisFailed

ScoringStarted
ScoringCompleted
ScoringFailed

RecommendationsStarted
RecommendationsGenerated
RecommendationsFailed

ReportGenerationStarted
ReportGenerated
ReportGenerationFailed
```

Возможны progress events:

```text
GitHubDataCollectionProgress
RepositoryDataCollectionProgress
AnalysisProgress
```

## Kafka topics

Command topics:

```text
repopulse.github.commands
repopulse.sandbox.commands
repopulse.analysis.commands
repopulse.scoring.commands
repopulse.recommendation.commands
repopulse.report.commands
```

Общий events topic для MVP:

```text
repopulse.analysis.events
```

DLQ topics:

```text
repopulse.<service>.commands.dlq
```

Отдельные Kafka topics для retry не используются. Каждый сервис выполняет повторные попытки обработки команды локально. DLQ хранит команды, обработка которых окончательно завершилась ошибкой; отдельного consumer или автоматического обработчика DLQ в MVP нет.

## Локальный retry и DLQ

Формат DLQ: [dlq.schema.json](../../project/backend/api/kafka/dlq.schema.json), [пример](../../project/backend/api/examples/dlq.json). Неразбираемое исходное сообщение хранится как Base64 исходных bytes.

Общий порядок обработки Kafka-команды сервисом:

```text
Command topic → сервис
    ↓
попытка выполнения
    ├── успех → сохранить результат → событие Completed / Generated
    │
    ├── временная ошибка → локальная задержка → следующая попытка
    │
    └── попытки исчерпаны / неповторяемая ошибка
           ├── исходная команда + сведения об ошибке → DLQ сервиса
           └── событие Failed → repopulse.analysis.events → Orchestrator
                                                               ↓
                                                             FAILED
```

Повторные попытки выполняет сервис, которому адресована команда: GitHub API Service, Repository Sandbox Service, Analysis Service, Scoring Service, Recommendation Service или Report Service. Orchestrator не переотправляет команды тяжёлых этапов для retry и не читает DLQ. Его локальный retry относится только к собственным синхронным gRPC-вызовам, например на этапе `PREPARING`.

Для каждого сервиса конфигурация задаёт максимальное число попыток, timeout отдельной операции и общий предел времени обработки команды с учётом задержек. Число попыток включает первый вызов. Временные ошибки повторяются с ограниченным exponential backoff и jitter; неповторяемые ошибки сразу завершают обработку. Retry внешних HTTP/gRPC-вызовов входит в тот же общий предел, чтобы вложенные retry не приводили к неограниченному числу запросов.

После окончательной ошибки сервис публикует соответствующее событие `Failed` из списка Kafka events с `analysis_id`, `attempts`, `error_code` и `error_message`. Orchestrator сохраняет ошибку этапа, число попыток в `analysis_steps.attempt`, переводит анализ в `FAILED` и прекращает запуск следующих этапов. Помещение команды в DLQ само по себе не заменяет событие об ошибке. Поздние события уже выполнявшихся параллельных этапов не возвращают завершённый с ошибкой анализ в `RUNNING` или `COMPLETED`.

DLQ-запись содержит исходное сообщение и данные для ручного разбора:

```text
original_message (исходный envelope команды, включая message_id и analysis_id)
source_topic
source_partition
source_offset
service
attempts
failed_at
error_code
error_message
```

Результаты анализа и credentials в DLQ не добавляются. Для команды, которую нельзя разобрать, сохраняются исходные данные и координаты Kafka-записи; событие `Failed` публикуется только если можно достоверно определить `analysis_id`. В противном случае Orchestrator завершает ожидающий этап по timeout.

Сервис подтверждает обработку Kafka-записи только после успешного сохранения результата и надёжной публикации события, а при окончательной ошибке — после подтверждённой публикации DLQ-записи и события `Failed`. При недоступности Kafka сообщение не считается обработанным. Повторная доставка после сбоя возможна; операции и обработка событий должны быть идемпотентными по `message_id`. Такая доставка не является отдельным механизмом retry через Kafka topics.


### Ответственность администраторов

Администраторы контролируют накопление сообщений в DLQ, изучают исходные команды и связанные логи по `analysis_id`, устраняют причины ошибок и принимают решение о дальнейших действиях. Retention DLQ задаётся конфигурацией Kafka так, чтобы оставалось время на ручной разбор.

Автоматическое чтение DLQ, автоматический replay и отдельный DLQ-сервис не планируются. Если после устранения причины нужен повторный анализ, он запускается как новый `AnalysisRun` обычным способом. Запись в DLQ не является очередью автоматического продолжения старого анализа.

## Хранилище

Database:

```text
orchestrator_db
```

Таблица `analysis_runs`:

```text
analysis_runs
├── id
├── user_id (nullable)
├── guest_session_id (nullable)
├── repository_owner
├── repository_name
├── repository_url
├── repository_revision
├── analysis_window
├── collector_profile_version
├── sandbox_id
├── sandbox_collection_id
├── status
├── current_stage
├── github_snapshot_id
├── analysis_snapshot_id
├── scoring_result_id
├── recommendation_result_id
├── report_id
├── created_at
├── started_at
├── completed_at
├── error_code
└── error_message
```

Ограничение таблицы: заполнено ровно одно поле владельца — `user_id` или `guest_session_id`. Идентификаторы относятся к данным Auth Service логически; межсервисные SQL-запросы и foreign keys в чужую database не используются. Владелец анализа фиксируется при создании и не меняется после входа гостя в аккаунт.

Таблица `analysis_steps`:

```text
analysis_steps
├── id
├── analysis_id
├── stage
├── status
├── attempt
├── progress_current
├── progress_total
├── started_at
├── completed_at
├── error_code
└── error_message
```

Таблица `outbox`:

```text
outbox
├── id
├── message_id
├── aggregate_id
├── topic
├── message_type
├── payload
├── created_at
└── published_at
```

Таблица `inbox`:

```text
inbox
├── message_id
├── processed_at
└── message_type
```

`Outbox` обеспечивает надёжную отправку Kafka-команд.

`Inbox` обеспечивает идемпотентную обработку повторно доставленных Kafka events.

## Redis

Orchestrator использует Redis для кэширования часто запрашиваемого состояния анализа:

```text
analysis:{requester_type}:{requester_id}:{analysis_id}:status
```

`requester_type` равен `user` или `guest`; `requester_id` берётся из проверенного контекста. Запись содержит идентификатор владельца и состояние анализа. Orchestrator сверяет владельца до возврата как кэшированного, так и сохранённого в database статуса. Кэш разных владельцев изолирован.

Source of truth остаётся в `orchestrator_db`.

---

# 4. GitHub API Service

## Ответственность

GitHub API Service отвечает только за platform-specific данные GitHub.

Он:

- обращается к GitHub REST API;
- получает repository metadata;
- получает Pull Requests;
- получает Reviews;
- получает участников и доступную статистику их вклада;
- получает Tags;
- получает Workflows;
- получает Workflow Runs;
- получает Jobs;
- получает Checks;
- получает правила защиты основной ветки и обязательные checks;
- получает Releases и release notes;
- обрабатывает pagination;
- обрабатывает GitHub rate limits;
- выполняет retry временных HTTP-ошибок;
- нормализует GitHub DTO во внутренние модели RepoPulse;
- отслеживает completeness каждого набора данных;
- создаёт `GitHubDataSnapshot`.

## GitHubDataSnapshot

`GitHubDataSnapshot` — нормализованный снимок platform-specific данных GitHub.

Логическая структура:

```text
GitHubDataSnapshot
├── Repository
├── RepositoryRevision
├── AnalysisWindow
├── PullRequests
├── Reviews
├── ContributorActivity
├── Tags
├── Workflows
├── WorkflowRuns
├── Jobs
├── Checks
├── BranchPolicies
├── Releases
├── CollectionMetadata
└── CollectedAt
```

Пример repository metadata:

```text
Repository
├── Owner
├── Name
├── DefaultBranch
├── UpdatedAt
└── PushedAt
```

Для каждого набора данных хранится состояние сбора:

```text
CollectionState
├── Status
├── IsComplete
├── PagesFetched
├── ItemsFetched
└── Errors
```

Важно различать:

```text
items = []
is_complete = true
→ данных действительно нет
```

и:

```text
items = []
is_complete = false
→ отсутствие данных не подтверждено
```

Остальные сервисы не должны зависеть от GitHub-specific DTO.

## gRPC-контракт

Используется на стадии `PREPARING` и для чтения сохранённого snapshot.

Полный контракт с сообщениями запросов и ответов: [github.proto](../../project/backend/api/proto/repopulse/github/v1/github.proto).

```protobuf
service GitHubAPIService {
  rpc ResolveRepositoryState(ResolveRepositoryStateRequest)
      returns (ResolveRepositoryStateResponse);

  rpc GetGitHubDataSnapshot(GetGitHubDataSnapshotRequest)
      returns (GitHubDataSnapshot);
}
```

`ResolveRepositoryState` логически возвращает:

```text
owner
repository
updated_at
default_branch
head_sha
```

Для head SHA выполняется дополнительное чтение основной ветки через API платформы; одного `repository.updated_at` недостаточно, чтобы закрепить checkout.

`GetGitHubDataSnapshot` принимает:

```text
github_snapshot_id
```

и возвращает сохранённый `GitHubDataSnapshot` сервису, которому он нужен.

## Kafka-контракт

Команда:

```text
CollectGitHubData
```

Пример:

```json
{
  "message_id": "uuid",
  "analysis_id": "A123",
  "type": "CollectGitHubData",
  "payload": {
    "owner": "example",
    "repository": "project",
    "branch": "main",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "window": {
      "from": "2026-09-03T00:00:00Z",
      "until": "2026-10-03T00:00:00Z"
    }
  }
}
```

Событие:

```text
GitHubDataCollected
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "GitHubDataCollected",
  "payload": {
    "github_snapshot_id": "GS123"
  }
}
```

## Хранилище

Database:

```text
github_db
```

Таблица `repositories`:

```text
repositories
├── id
├── owner
├── name
├── canonical_url
├── default_branch
├── updated_at
├── pushed_at
└── created_at
```

Таблица `github_snapshots`:

```text
github_snapshots
├── id
├── analysis_id
├── repository_id
├── repository_revision
├── analysis_window
├── collected_at
└── status
```

Нормализованные сущности:

```text
pull_requests
reviews
contributor_activity
tags
workflows
workflow_runs
jobs
checks
branch_policies
releases
```

Каждая сущность связана с `github_snapshot_id`.

Для MVP сохраняются автор PR, пользователь, выполнивший merge, даты создания/слияния, авторы, состояния и даты reviews, результаты CI-запусков, эффективные требования review/checks для основной ветки, теги и тексты release notes. Участники и статистика вклада нормализуются в `ContributorActivity` с количеством коммитов, additions/deletions, периодом и статусом полноты. Источник — GitHub commits/statistics API; статистика с другим периодом или неполными данными не выдаётся за точную выборку окна AnalysisRun. См. [GitHub repository statistics](https://docs.github.com/en/rest/metrics/statistics). `BranchPolicies` учитывает доступные branch protection и rulesets. Недостаток прав API или неполный сбор правил означает `unavailable`/`partial`, а не отсутствие защиты. См. [GitHub branch protection API](https://docs.github.com/en/rest/branches/branch-protection) и [rules API](https://docs.github.com/en/rest/repos/rules).

Все временные выборки используют то же окно UTC, что и сбор данных из sandbox. Snapshot сохраняет имя основной ветки и SHA, зафиксированные Orchestrator на этапе `PREPARING`. Данные PR/reviews/CI и текущие правила платформы не являются атомарным Git-снимком: сохраняются их период и время сбора. Источник данных платформы — GitHub API Service.

Таблица `collection_states`:

```text
collection_states
├── id
├── github_snapshot_id
├── resource_type
├── collection_status
├── is_complete
├── pages_fetched
├── items_fetched
└── collected_at
```

Таблица `collection_errors`:

```text
collection_errors
├── id
├── collection_state_id
├── provider
├── resource
├── endpoint
├── http_status
├── error_code
└── message
```

---

# 5. Repository Sandbox Service

## Ответственность

Repository Sandbox Service получает `CollectRepositoryData` от Orchestrator через Kafka, создаёт временную изолированную среду, клонирует репозиторий на зафиксированный SHA, выполняет локальный сбор и запускает SonarScanner. Готовые структурированные данные Analysis Service получает через gRPC; код и полный checkout между сервисами не передаются.

Внутри sandbox находятся:

```text
Sandbox S123
├── Git и checkout /workspace/repository
├── RepoPulse collector: файлы, конфигурации и git log
├── SonarScanner CLI
└── временные результаты и рабочие каталоги инструментов
```

Сервис ограничивает CPU, RAM, disk, PID и время выполнения, запускает процессы от non-root и контролирует сеть. Sandbox сохраняется до получения и сохранения результатов Analysis Service, затем удаляется по `ReleaseSandbox`; забытые среды удаляются по TTL. Итоговые метрики, интерпретацию проблем и Repo Health Score внутри sandbox не рассчитывают.

## Изоляция и сеть

```text
containerd → gVisor (runsc) → isolated sandbox

COLLECTING, clone/fetch: Sandbox → GitHub
COLLECTING, scanner:     Sandbox → SonarQube Server (HTTPS)
                        остальной исходящий доступ запрещён
```

Используются read-only root filesystem, отдельные writable-каталоги для checkout/результатов/кэша scanner, drop capabilities и hard timeout. Git-история для выбранного периода загружается при подготовке; сбор не делает повторный pull и не меняет SHA. SonarScanner и необходимые runtime-компоненты установлены в доверенном образе заранее.

Сборщик читает файлы и Git-историю. Он не выполняет код репозитория, build, тесты, package scripts или CI-конфигурацию и не устанавливает зависимости репозитория. Статический анализ выполняет SonarScanner с параметрами, сформированными RepoPulse. Настройки из репозитория не должны переопределять адрес сервера, токен, scope или включать выполнение сторонних команд.

## Данные sandbox для MVP

Локальный сбор ограничен следующими проверками. Дополнительные сведения о работе команды и платформы поступают через GitHub API Service, а результаты статического анализа — из SonarQube Server.

| Проверка | Что возвращает sandbox | Что делает Analysis Service |
|---|---|---|
| Наличие документации | Признаки README, папки/файлов docs и основных supporting-файлов с относительными путями | Фиксирует наличие документации; полнота, качество текста и актуальность по датам не оцениваются |
| Наличие тестов | Распознаваемые файлы тестов и конфигурации тестовых инструментов | Фиксирует наличие; тесты не запускаются, coverage не рассчитывается |
| Наличие CI/CD | Пути конфигураций, например `.github/workflows`, и факт их обнаружения | Фиксирует наличие конфигурации; успешность запусков и обязательность checks берёт из GitHub API |
| `.gitignore` и его структура | Наличие файла, число активных ignore-правил и локальная проверка исключения `.env` по правилам Git | Показывает наличие/непустой набор правил и исключение `.env`; не утверждает, что все необходимые файлы исключены |
| Отсутствие `.env` | Пути отслеживаемых файлов с точным basename `.env` в текущем checkout | При обнаружении формирует finding. `.env.example`/`.env.sample` не считаются `.env`; содержимое файлов не возвращается |
| Частота коммитов | Дневные количества коммитов из `git log` за заданный период UTC | Рассчитывает commits/day и commits/week; нулевые дни достраивает только при полном сборе периода |
| Статический анализ | Результат запуска SonarScanner: project key, идентификатор серверной задачи, версия scanner | Дожидается обработки на SonarQube Server и получает метрики/issues через его API |

`mvp-v1` задаёт поддерживаемые экосистемы и правила обнаружения файлов. Документация проверяется по README, docs, LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY и issue/PR templates; возвращается наличие, а не оценка содержания. Отсутствие `.env` проверяется только в текущем tracked checkout, не во всей истории. Проверка ignore-правил использует Git semantics для корневого `.env` и обнаруженных путей; наличие ignore-правила не отменяет finding, если `.env` уже tracked.

SonarQube — инструмент статического анализа MVP. Сведения об участниках, review, результатах CI, защите ветки, тегах и релизах получает GitHub API Service.

## gRPC-контракт с Analysis Service

Отдельный файл контракта: [sandbox.proto](../../project/backend/api/proto/repopulse/sandbox/v1/sandbox.proto). Порядок фиксации контрактов MVP: [mvp-contracts.md](mvp-contracts.md).

На этапе `COLLECTING` Orchestrator параллельно отправляет `CollectGitHubData` и `CollectRepositoryData` через Kafka. Sandbox Service выполняет clone, локальные проверки и SonarScanner, сохраняет результат и публикует `RepositoryDataCollected`. Analysis Service получает задание только после готовности обоих источников.

```text
Orchestrator
  ├── Kafka → GitHub API Service: CollectGitHubData
  └── Kafka → Sandbox Service: CollectRepositoryData
               clone → локальный collector → SonarScanner
                         ↓
               RepositoryDataCollected(S123, RC123)

Orchestrator: GitHubDataCollected + RepositoryDataCollected
                         ↓
                 Kafka: AnalyzeRepository
                         ↓
Analysis Service
  ├── gRPC → GitHub API Service.GetGitHubDataSnapshot(GS123)
  ├── gRPC → Sandbox Service.GetRepositoryData(S123, RC123)
  └── SonarQube API: дождаться задачи и получить результаты
                         ↓
                  AnalysisSnapshot
                         ↓
          gRPC → Sandbox Service.ReleaseSandbox(S123)
```

Sandbox Service запускает установленные инструменты через containerd внутри нужной sandbox. gRPC между Analysis Service и Sandbox Service используется для чтения готовых данных и освобождения среды:

```protobuf
syntax = "proto3";
package repopulse.sandbox.v1;

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";
import "repopulse/common/v1/common.proto";

service RepositorySandboxService {
  rpc GetRepositoryData(RepositoryCollectionRef)
      returns (stream RepositoryDataChunk);
  rpc ReleaseSandbox(ReleaseSandboxRequest)
      returns (google.protobuf.Empty);
}

message RepositoryCollectionRef {
  string analysis_id = 1;
  string sandbox_id = 2;
  string collection_id = 3;
}

message ReleaseSandboxRequest {
  string analysis_id = 1;
  string sandbox_id = 2;
}

message RepositoryDataChunk {
  uint64 sequence = 1;
  oneof payload {
    RepositoryDataHeader header = 2;
    CommitActivity activity = 3;
    RepositoryFileFacts file_facts = 4;
    GitignoreFacts gitignore = 6;
    SonarScanResult sonar_scan = 7;
    CollectionSummary summary = 8;
  }
}

message RepositoryDataHeader {
  string collection_id = 1;
  string analysis_id = 2;
  string sandbox_id = 3;
  string revision = 4;
  string branch = 5;
  string profile_version = 6;
  string collector_version = 7;
  repopulse.common.v1.AnalysisWindow window = 8;
  google.protobuf.Timestamp collected_at = 9;
  bool shallow = 10;
}

message CommitActivity {
  string date_utc = 1; // YYYY-MM-DD.
  uint64 commit_count = 2;
}

enum DetectionState {
  DETECTION_STATE_UNSPECIFIED = 0;
  DETECTION_STATE_DETECTED = 1;
  DETECTION_STATE_NOT_DETECTED = 2;
  DETECTION_STATE_UNKNOWN = 3;
  DETECTION_STATE_NOT_APPLICABLE = 4;
}

message RepositoryFileFacts {
  optional bool has_readme = 1;
  optional bool has_docs = 2;
  optional bool has_supporting_docs = 3;
  optional bool has_tests = 4; // Найдены тестовые файлы.
  optional bool has_test_config = 5;
  optional bool has_ci_config = 6;
  repeated string documentation_paths = 7;
  repeated string test_paths = 8;
  repeated string ci_config_paths = 9;
  repeated string tracked_env_paths = 10;
}

message GitignoreFacts {
  DetectionState file_presence = 1;
  optional uint64 active_rule_count = 2; // Без пустых строк/комментариев.
  DetectionState env_ignore_rule = 3; // Семантика Git, не поиск подстроки.
  repeated string paths = 4; // Проверенные .gitignore относительно корня checkout.
}

message SonarScanResult {
  repopulse.common.v1.DataStatus status = 1;
  string project_key = 2;
  string ce_task_id = 3; // Идентификатор фоновой задачи SonarQube.
  string scanner_version = 4;
  string error_code = 5;
}

message SectionState {
  string section = 1;
  repopulse.common.v1.DataStatus status = 2;
  uint64 items_examined = 3;
  uint64 items_skipped = 4;
  repeated string reason_codes = 5;
}

message CollectionSummary {
  repeated SectionState sections = 1;
  uint64 chunks_before_summary = 2;
}
```

### Секции и значения

| Секция | Поля результата |
|---|---|
| `documentation` | `RepositoryFileFacts`: `has_readme`, `has_docs`, `has_supporting_docs`, `documentation_paths` |
| `tests` | `RepositoryFileFacts`: `has_tests`, `has_test_config`, `test_paths` |
| `ci` | `RepositoryFileFacts`: `has_ci_config`, `ci_config_paths`; фактическое выполнение приходит из GitHub API |
| `gitignore` | `GitignoreFacts`: наличие, число активных правил, исключение `.env` |
| `env_files` | `RepositoryFileFacts.tracked_env_paths`: пути найденных tracked `.env` |
| `git_activity` | `CommitActivity`: дневные счётчики из `git log` |
| `sonar_scan` | `SonarScanResult`: результат CLI и ссылка на серверную обработку |

`RepositoryFileFacts` содержит фиксированный набор полей MVP и передаётся одним chunk после header. Списки путей ограничиваются бюджетом chunk; при усечении соответствующая секция помечается `PARTIAL`. `has_supporting_docs` означает наличие хотя бы одного LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY или issue/PR template; обнаруженные пути включаются в `documentation_paths`. `has_tests` означает наличие распознанных тестовых файлов, `has_test_config` — конфигурации тестового инструмента.

Для `optional bool` значение `true` означает обнаруженный признак, `false` — подтверждённое отсутствие, отсутствие значения — невозможность определить результат. `false` допустим только после полной проверки scope этого признака; ограничения секции отражаются в `CollectionSummary`. Например, найденный README даёт `has_readme = true`, даже если остальные supporting-файлы удалось проверить лишь частично. Наличие README определяется существованием файла; его содержательность и качество не оцениваются.

Пути во всех полях относительны корню checkout. Непустой `tracked_env_paths` означает найденный tracked `.env`; пустой список подтверждает отсутствие только при `COMPLETE` секции `env_files`. Аналогично пустой список остальных путей при неполном сборе не доказывает отсутствие файлов. Содержимое `.env` не читается локальным сборщиком и исключается из отправки scanner. SHA всего checkout передаётся один раз в header. `DetectionState` используется для конкретных проверок `GitignoreFacts`; `NOT_DETECTED` допустим только после полной проверки соответствующего scope, неизвестный результат обозначается `UNKNOWN`.

### Окно и полнота

`from < until`; длительность выбирается конфигурацией, например 30 дней. Окно фиксируется в AnalysisRun и передаётся GitHub API Service и sandbox. История относится к коммитам, достижимым из `revision` основной ветки, с committer timestamp в `[from, until)` UTC; для частоты учитываются все коммиты, включая merge, каждый SHA один раз.

`git log` исполняется с фиксированными аргументами без shell-команд пользователя; даты и счётчики возвращаются без patch и содержимого файлов. Возможности Git описаны в [git log](https://git-scm.com/docs/git-log). Shallow history, лимит коммитов/файлов/размеров или timeout дают `PARTIAL` соответствующей секции. Полнота периода не выводится из одного старого видимого коммита: [git clone](https://git-scm.com/docs/git-clone).

Пустой результат при `COMPLETE` означает «не найдено в scope», при `PARTIAL`/`UNAVAILABLE` — «данных недостаточно». Summary содержит состояния всех семи секций.

### Выполнение, retry и освобождение

Получив `CollectRepositoryData`, Sandbox Service публикует `RepositoryDataCollectionStarted` и выполняет работу в своей sandbox. После локального сбора и успешной загрузки отчёта SonarScanner он сохраняет неизменяемые факты, метаданные scanner и `collection_id`, затем надёжно публикует `RepositoryDataCollected`. Это событие означает доступность данных по gRPC; серверная обработка SonarQube ещё может продолжаться. Частичные проверки отражаются в summary. Неподдерживаемый язык даёт `UNAVAILABLE`/`NOT_APPLICABLE` секции scanner, а не подтверждение успешного статического анализа.

Sandbox Service обрабатывает повторную доставку Kafka-команды идемпотентно по `message_id`: готовый результат и событие используются повторно. Ключ с другими параметрами отклоняется. Локальный retry clone/collector/scanner принадлежит Sandbox Service и ограничен общим бюджетом команды. Если отчёт scanner уже загружен и его `ce_task_id` сохранён, повторная публикация события не запускает scanner заново. Для новой попытки scanner сервис создаёт отдельный Sonar-проект, чтобы поздняя обработка предыдущего отчёта не перезаписала выбранный результат.

После окончательной ошибки Sandbox Service отправляет исходную команду в `repopulse.sandbox.commands.dlq` и публикует `RepositoryDataCollectionFailed`. Orchestrator переводит AnalysisRun в `FAILED` и не запускает Analysis Service. Недоступные вспомогательные проверки могут дать частичный готовый результат; техническая ошибка clone или scanner для поддерживаемого проекта завершает сбор ошибкой. Состояние серверной задачи после получения данных повторно проверяет Analysis Service без запуска инструментов sandbox.

`GetRepositoryData` сверяет связь `analysis_id`/`sandbox_id`/`collection_id` и полномочия внутреннего клиента. Ошибки контракта: `INVALID_ARGUMENT` для параметров, `NOT_FOUND` для неизвестной/истёкшей среды или результата, `FAILED_PRECONDITION` при чтении незавершённого сбора. После `RepositoryDataCollected` результат должен быть доступен; кратковременная ошибка transport обрабатывается локальным retry чтения Analysis Service и не создаёт новую задачу сбора.

Первый chunk — header, последний — summary, `sequence` начинается с 0; факты идут в стабильном порядке. Максимум chunk — 256 KiB, общий лимит задаёт профиль. При обрыве клиент перечитывает неизменяемый результат с начала и дедуплицирует `(collection_id, sequence)`. Без summary поток не считается полным. При достижении лимита сохраняются целые факты и `PARTIAL`; header, summary и состояния секций резервируются в бюджете.

Состояние обработки команды, ключи идемпотентности и результаты хранятся временно вне checkout, без постоянной database. TTL покрывает сбор, ожидание второго источника, очередь Analysis Service, обработку SonarQube и retry; hard timeout предотвращает бесконечное удержание среды. Потеря sandbox после рестарта даёт `NOT_FOUND` и ошибку этапа, без скрытого повторного clone из Analysis Service.

Analysis Service сохраняет локальные факты и результаты SonarQube в `AnalysisSnapshot`, затем вызывает идемпотентный `ReleaseSandbox`. Он останавливает оставшиеся процессы и удаляет checkout/временные результаты; повторное освобождение возвращает успех. При терминальной ошибке агрегации Analysis Service также освобождает среду. Если Analysis Service не запущен из-за ошибки параллельного GitHub-сбора, Sandbox Service удаляет среду по TTL. Неудачная очистка не отменяет сохранённый snapshot.

## Kafka-контракт

Команда `CollectRepositoryData`:

```json
{
  "message_id": "uuid",
  "analysis_id": "A123",
  "type": "CollectRepositoryData",
  "payload": {
    "owner": "example",
    "repository": "project",
    "branch": "main",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "profile_version": "mvp-v1",
    "window": {
      "from": "2026-09-03T00:00:00Z",
      "until": "2026-10-03T00:00:00Z"
    }
  }
}
```

Событие `RepositoryDataCollected`:

```json
{
  "analysis_id": "A123",
  "type": "RepositoryDataCollected",
  "payload": {
    "sandbox_id": "S123",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "branch": "main",
    "expires_at": "2026-10-03T12:00:00Z",
    "collection_id": "RC123"
  }
}
```

---


# 6. Analysis Service

## Ответственность

Analysis Service агрегирует результаты из трёх источников:

```text
RepositoryData (Git/файлы из sandbox)
             +
GitHubDataSnapshot (данные платформы)
             +
SonarQube results (статический анализ)
             ↓
       AnalysisSnapshot
```

Он получает готовые данные GitHub и sandbox по gRPC, ждёт серверной обработки SonarQube, нормализует результаты и формирует metrics, findings, observations и completeness. Запуск clone, локального collector и SonarScanner выполняет Sandbox Service по Kafka-команде Orchestrator. Repo Health Score рассчитывает следующий Scoring Service.

## Обработка данных MVP

1. Получает `AnalyzeRepository` от Orchestrator после событий готовности GitHub и sandbox. Команда содержит `github_snapshot_id`, `sandbox_id`, `sandbox_collection_id`, SHA, профиль и окно анализа.
2. Получает `GitHubDataSnapshot` и готовый поток `GetRepositoryData` через gRPC. Сверяет AnalysisRun, SHA, профиль, окно, header и summary; локальные проверки и scanner к этому моменту выполнены Sandbox Service.
3. Из дневных счётчиков рассчитывает среднюю частоту за период: `commit_count / duration_days` и `commit_count / duration_weeks`. Признаки файлов превращает в наблюдения; найденный tracked `.env` — в finding с путём, без содержимого.
4. По `ce_task_id` ждёт именно соответствующую задачу SonarQube. После её успешного завершения получает метрики и issues через API с привязкой к project/analysis и сохраняет идентификатор Sonar-анализа. CLI success до завершения серверной задачи не означает готовность результатов. Неуспешный Quality Gate сам по себе не является технической ошибкой RepoPulse: низкое качество кода — результат анализа.
5. Агрегирует остальные доступные факты из GitHub API Service: метаданные, участников и статистику вклада, PR/reviews, независимое review и self-merge, даты review, правила защиты/обязательные checks, фактические результаты CI, теги и releases/release notes. Для каждого показателя сохраняются период, источник и ограничения полноты.
6. Сохраняет единый `AnalysisSnapshot` в `analysis_db`, освобождает sandbox и публикует `AnalysisCompleted(analysis_snapshot_id)`. Дальше Orchestrator запускает `Scoring → Recommendation → Report` как раньше.

Недоступные данные платформы, неподдерживаемые языки или неполный локальный сбор отражаются в completeness и не превращаются в нули/ложное отсутствие. Ошибка серверной задачи SonarQube или агрегации после локального retry приводит к `AnalysisFailed` и DLQ Analysis Service по прежнему правилу. Ошибку запуска scanner обрабатывает Sandbox Service до события готовности. API-данные платформы имеют собственное время сбора и не образуют атомарный snapshot с Git checkout.

## SonarQube

SonarQube Server — постоянный внешний компонент; SonarScanner CLI запускает Sandbox Service в рамках Kafka-команды `CollectRepositoryData` от Orchestrator.

```text
Orchestrator → Kafka → Sandbox Service
                              ↓ containerd exec
                         SonarScanner
                              ↓ отчёт анализа
                         SonarQube Server
                              ↑ API: task status, metrics, issues
                         Analysis Service
```

Адрес SonarQube, credentials и разрешённые параметры принадлежат конфигурации RepoPulse. Sandbox Service подготавливает Sonar-проект с key, уникальным для AnalysisRun и попытки scanner; использует внутренний технический токен с необходимыми правами создания проекта/анализа и не пишет его в логи. Рабочие файлы scanner находятся вне checkout. Сборщик возвращает идентификатор server task из метаданных scanner, а не произвольный URL для обращения Analysis Service.

Завершение CLI означает загрузку отчёта; сервер обрабатывает его асинхронно. См. [SonarScanner CLI](https://docs.sonarsource.com/sonarqube-server/analyzing-source-code/scanners/sonarscanner) и [Background tasks](https://docs.sonarsource.com/sonarqube-server/analyzing-source-code/background-tasks). Возвращённый `ce_task_id` используется для ожидания и сопоставления результатов текущему запуску.

MVP не выполняет build/тесты и не генерирует coverage. Анализируются языки и проекты, которые поддерживает настроенный SonarQube без выполнения кода репозитория; необходимость артефактов сборки или неподдерживаемый язык отражается как недоступная часть анализа. Успешный scanner не означает полную проверку всех языков репозитория; фактические ограничения сохраняются в completeness.


## AnalysisSnapshot

`AnalysisSnapshot` — нормализованный набор технических фактов о репозитории.

```text
AnalysisSnapshot
├── Metrics
├── Findings
├── Observations
├── Completeness
└── CreatedAt
```

`Metric` — измеримый показатель:

```json
{
  "name": "ci_success_rate",
  "value": 0.82,
  "category": "ci_cd",
  "source": "github"
}
```

`Finding` — конкретная обнаруженная проблема:

```json
{
  "category": "security",
  "severity": "high",
  "source": "sonarqube",
  "message": "Potential vulnerability",
  "file": "internal/auth.go",
  "line": 42
}
```

`Observation` — факт или характеристика, которая сама по себе не обязательно является проблемой.

## gRPC-контракт

Analysis Service предоставляет read-метод для получения сохранённого snapshot:

Полный контракт с сообщениями запросов и ответов: [analysis.proto](../../project/backend/api/proto/repopulse/analysis/v1/analysis.proto).

```protobuf
service AnalysisService {
  rpc GetAnalysisSnapshot(GetAnalysisSnapshotRequest)
      returns (AnalysisSnapshot);
}
```

Запрос содержит:

```text
analysis_snapshot_id
```

## Kafka-контракт

Команда:

```text
AnalyzeRepository
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "AnalyzeRepository",
  "payload": {
    "sandbox_id": "S123",
    "github_snapshot_id": "GS123",
    "revision": "0123456789abcdef0123456789abcdef01234567",
    "profile_version": "mvp-v1",
    "window": {
      "from": "2026-09-03T00:00:00Z",
      "until": "2026-10-03T00:00:00Z"
    },
    "sandbox_collection_id": "RC123"
  }
}
```

После получения команды Analysis Service запрашивает `GitHubDataSnapshot` у GitHub API Service через gRPC по `github_snapshot_id`.

Для sandbox он вызывает только `GetRepositoryData`, используя `sandbox_collection_id` из команды. Запуск сбора уже выполнен по Kafka-команде Orchestrator. Полный `RepositoryData` по Kafka не передаётся. После сохранения `AnalysisSnapshot` Analysis Service освобождает sandbox через `ReleaseSandbox` и публикует `AnalysisCompleted`.

Событие:

```text
AnalysisCompleted
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "AnalysisCompleted",
  "payload": {
    "analysis_snapshot_id": "AS123"
  }
}
```

Полный `AnalysisSnapshot` через Kafka не передаётся.

## Хранилище

Database:

```text
analysis_db
```

Таблица `analysis_snapshots`:

```text
analysis_snapshots
├── id
├── analysis_id
├── repository_revision
├── sandbox_collection_id
├── collector_profile_version
├── analysis_window
├── completeness
└── created_at
```

Таблица `metrics`:

```text
metrics
├── id
├── analysis_snapshot_id
├── category
├── name
├── numeric_value
├── text_value
└── source
```

Таблица `findings`:

```text
findings
├── id
├── analysis_snapshot_id
├── category
├── severity
├── source
├── message
├── file_path
└── line
```

Таблица `observations`:

```text
observations
├── id
├── analysis_snapshot_id
├── category
├── source
└── description
```

Дополнительно сохраняются `source_collection_states` и собранные `repository_facts`, связанные с `analysis_snapshot_id`. Сохраняются структурированные факты и результаты SonarQube; для локальных фактов достаточно списка путей и общего SHA snapshot. Итоговый отчёт MVP содержит оценки, краткие выводы, рекомендации и ограничения полноты данных. Sandbox не служит постоянным хранилищем результатов.

---

# 7. Scoring Service

## Ответственность

Scoring Service рассчитывает оценку состояния репозитория.

Он получает:

```text
AnalysisSnapshot
```

и формирует:

```text
ScoringResult
```

Он:

- рассчитывает score по категориям;
- рассчитывает общий Repo Health Score;
- формирует объяснение оценки;
- использует AI/LLM Provider;
- хранит model/prompt/schema versions.

Он не ходит в GitHub, не читает repository и не запускает SonarQube.

## ScoringResult

```text
ScoringResult
├── OverallScore
├── CategoryScores
├── Explanation
├── ModelVersion
├── PromptVersion
├── SchemaVersion
└── CreatedAt
```

Пример:

```json
{
  "overall_score": 74,
  "category_scores": [
    {
      "category": "documentation",
      "score": 90,
      "explanation": "..."
    },
    {
      "category": "ci_cd",
      "score": 62,
      "explanation": "..."
    }
  ]
}
```

## gRPC-контракт

Схема структурированного ответа LLM: [scoring-result.schema.json](../../project/backend/api/llm/scoring-result.schema.json). Идентификаторы и версии результата добавляет сервис после проверки ответа; методика оценки согласуется при реализации.

Scoring Service предоставляет read-метод:

Полный контракт с сообщениями запросов и ответов: [scoring.proto](../../project/backend/api/proto/repopulse/scoring/v1/scoring.proto).

```protobuf
service ScoringService {
  rpc GetScoringResult(GetScoringResultRequest)
      returns (ScoringResult);
}
```

Запрос содержит:

```text
scoring_result_id
```

## Kafka-контракт

Команда:

```text
CalculateScore
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "CalculateScore",
  "payload": {
    "analysis_snapshot_id": "AS123"
  }
}
```

После получения команды Scoring Service вызывает:

```text
Analysis Service.GetAnalysisSnapshot(analysis_snapshot_id)
```

по gRPC, получает полный `AnalysisSnapshot` и выполняет расчёт.

Событие:

```text
ScoringCompleted
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "ScoringCompleted",
  "payload": {
    "scoring_result_id": "SC123"
  }
}
```

Полный `ScoringResult` через Kafka не передаётся.

## Хранилище

Database:

```text
scoring_db
```

Таблица `scoring_results`:

```text
scoring_results
├── id
├── analysis_id
├── analysis_snapshot_id
├── overall_score
├── explanation
├── model_version
├── prompt_version
├── schema_version
└── created_at
```

Таблица `category_scores`:

```text
category_scores
├── id
├── scoring_result_id
├── category
├── score
└── explanation
```

---

# 8. Recommendation Service

## Ответственность

Recommendation Service формирует рекомендации по улучшению репозитория.

Он получает:

```text
AnalysisSnapshot
+
ScoringResult
```

и формирует:

```text
RecommendationResult
```

Рекомендации содержат:

- priority;
- action;
- expected effect;
- complexity;
- связь с конкретными findings.

Для генерации используется AI/LLM Provider.

## RecommendationResult

```text
RecommendationResult
├── Recommendations
├── ModelVersion
├── PromptVersion
├── SchemaVersion
└── CreatedAt
```

Пример рекомендации:

```json
{
  "priority": "high",
  "action": "Добавить обязательный запуск тестов в CI для pull request",
  "expected_effect": "Снизит вероятность попадания непроверенных изменений в основную ветку",
  "complexity": "small",
  "related_finding_ids": [
    "finding-42"
  ]
}
```

## gRPC-контракт

Схема структурированного ответа LLM: [recommendations.schema.json](../../project/backend/api/llm/recommendations.schema.json). Сервис проверяет ссылки на исходные findings и сам назначает ID рекомендаций.

Recommendation Service предоставляет read-метод:

Полный контракт с сообщениями запросов и ответов: [recommendation.proto](../../project/backend/api/proto/repopulse/recommendation/v1/recommendation.proto).

```protobuf
service RecommendationService {
  rpc GetRecommendationResult(GetRecommendationResultRequest)
      returns (RecommendationResult);
}
```

Запрос содержит:

```text
recommendation_result_id
```

## Kafka-контракт

Команда:

```text
GenerateRecommendations
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "GenerateRecommendations",
  "payload": {
    "analysis_snapshot_id": "AS123",
    "scoring_result_id": "SC123"
  }
}
```

После получения команды Recommendation Service синхронно получает необходимые данные:

```text
gRPC → Analysis Service.GetAnalysisSnapshot(analysis_snapshot_id)
gRPC → Scoring Service.GetScoringResult(scoring_result_id)
```

Событие:

```text
RecommendationsGenerated
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "RecommendationsGenerated",
  "payload": {
    "recommendation_result_id": "REC123"
  }
}
```

Полный `RecommendationResult` через Kafka не передаётся.

## Хранилище

Database:

```text
recommendation_db
```

Таблица `recommendation_results`:

```text
recommendation_results
├── id
├── analysis_id
├── analysis_snapshot_id
├── scoring_result_id
├── model_version
├── prompt_version
├── schema_version
└── created_at
```

Таблица `recommendations`:

```text
recommendations
├── id
├── recommendation_result_id
├── priority
├── action
├── expected_effect
└── complexity
```

Таблица `recommendation_findings`:

```text
recommendation_findings
├── recommendation_id
└── finding_id
```

---

# 9. Report Service

## Ответственность

Report Service является владельцем итоговых отчётов пользователей и гостевых сессий.

Он:

- формирует итоговый Report;
- сохраняет отчёт;
- предоставляет готовые отчёты;
- предоставляет авторизованному пользователю историю его отчётов;
- предоставляет авторизованному пользователю его историю по конкретному репозиторию;
- проверяет владельца отчёта для пользователя и гостевой сессии;
- формирует PDF по запросу авторизованного владельца отчёта;
- ищет reusable report в рамках того же владельца;
- хранит `repository.updated_at`;
- создаёт новый immutable Report на каждый новый анализ.

Он не рассчитывает score и не генерирует рекомендации.

## Report

Итоговая модель:

```text
Report
├── ID
├── AnalysisID
├── UserID (nullable)
├── GuestSessionID (nullable)
├── Repository
├── RepositoryUpdatedAt
├── RepositoryRevision
├── AnalysisWindow
├── CollectorProfileVersion
├── OverallScore
├── CategoryScores
├── Findings
├── Recommendations
├── Completeness
├── GeneratedAt
└── SchemaVersion
```

`RepositoryRevision`, `AnalysisWindow` и `CollectorProfileVersion` переносятся из AnalysisSnapshot и описывают, какой checkout и период анализировались. Недоступная оценка передаётся как `null` в HTTP и отсутствие optional-поля в gRPC; неизвестный результат не равен нулю.

Отчёт является immutable:

```text
новый анализ → новый Report
старый Report → не изменяется
```

Отчёт принадлежит ровно одному владельцу: пользователю или гостевой сессии. Владелец фиксируется при создании; вход и регистрация гостя его не меняют. Гостевой отчёт не включается в историю аккаунта и не предоставляется этому аккаунту для скачивания PDF.

## gRPC-контракт

Полный контракт с сообщениями запросов и ответов: [report.proto](../../project/backend/api/proto/repopulse/report/v1/report.proto).

```protobuf
service ReportService {
  rpc GetReport(GetReportRequest)
      returns (GetReportResponse);

  rpc GetReportPdf(GetReportPdfRequest)
      returns (stream ReportPdfChunk);

  rpc GetUserReports(GetUserReportsRequest)
      returns (GetUserReportsResponse);

  rpc GetRepositoryReports(GetRepositoryReportsRequest)
      returns (GetRepositoryReportsResponse);

  rpc FindReusableReport(FindReusableReportRequest)
      returns (FindReusableReportResponse);
}
```

При генерации отчёта Report Service получает полные данные не через Kafka, а через gRPC у сервисов-владельцев:

```text
gRPC → GitHub API Service.GetGitHubDataSnapshot(github_snapshot_id)
gRPC → Analysis Service.GetAnalysisSnapshot(analysis_snapshot_id)
gRPC → Scoring Service.GetScoringResult(scoring_result_id)
gRPC → Recommendation Service.GetRecommendationResult(recommendation_result_id)
```

`GetReport` принимает:

```text
requester
report_id
```

и проверяет совпадение владельца с `Requester`. Проверка обязательна при ответе как из PostgreSQL, так и из Redis.

`GetReportPdf` принимает `user_id` из проверенного пользовательского контекста и `report_id`. Report Service проверяет, что отчёт принадлежит этому `user_id`, затем формирует PDF из сохранённых данных и возвращает поток `ReportPdfChunk { bytes data }`. Отчёт гостевой сессии не проходит эту проверку. PDF создаётся по запросу; дополнительный анализ репозитория не запускается.

`GetUserReports` принимает `user_id` и параметры пагинации и возвращает только отчёты этого пользователя. Гостевой контекст не допускается.

`GetRepositoryReports` принимает `user_id`, repository и параметры пагинации и возвращает историю этого пользователя для конкретного репозитория. Гостевой контекст не допускается. Report Service повторно проверяет тип инициатора для истории и PDF, даже если ограничение уже применено в API Service.

`FindReusableReport` принимает:

```text
requester
repository
repository_updated_at
```

и возвращает подходящий `report_id`, если такой отчёт уже существует у того же владельца. Пользовательский отчёт ищется по `user_id`, гостевой — по `guest_session_id`. Новый пользовательский анализ не переиспользует гостевой отчёт.

## Kafka-контракт

Команда:

```text
GenerateReport
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "GenerateReport",
  "payload": {
    "requester": {
      "user_id": "U123"
    },
    "github_snapshot_id": "GS123",
    "analysis_snapshot_id": "AS123",
    "scoring_result_id": "SC123",
    "recommendation_result_id": "REC123"
  }
}
```

Для гостевого анализа поле `requester` содержит `{"guest_session_id": "G123"}` вместо `user_id`. Orchestrator копирует владельца из сохранённого `AnalysisRun` в команду и сохраняет её в outbox. Credentials, JWT, cookie и refresh-токены через Kafka не передаются.

Команда содержит идентификатор владельца и только идентификаторы результатов предыдущих этапов. После её получения Report Service забирает полные данные через gRPC у соответствующих сервисов и сохраняет владельца в `reports`. Отсутствующий или неоднозначный владелец приводит к отклонению команды, а не к созданию публичного отчёта.

После формирования и сохранения отчёта Report Service публикует:

```text
ReportGenerated {
    analysis_id,
    report_id
}
```

Пример:

```json
{
  "analysis_id": "A123",
  "type": "ReportGenerated",
  "payload": {
    "report_id": "R123"
  }
}
```

Analysis Orchestrator получает событие и переводит анализ:

```text
REPORTING → COMPLETED
```

Также Orchestrator сохраняет:

```text
analysis_run.report_id = R123
```

## Получение готового отчёта

Генерация отчёта выполняется асинхронно через Kafka, а чтение уже сформированного отчёта — синхронно через gRPC.

Frontend гостя или авторизованного пользователя узнаёт о завершении своего анализа через polling с соответствующими credentials:

```text
GET /api/v1/analyses/{analysis_id}
```

После завершения API возвращает:

```json
{
  "analysis_id": "A123",
  "status": "completed",
  "stages": [],
  "report_id": "R123"
}
```

После этого frontend получает свой отчёт отдельным запросом. Гость предъявляет гостевую cookie, пользователь — access JWT; API передаёт проверенный `Requester` в Report Service:

```text
Frontend
    ↓ HTTP
GET /api/v1/reports/{report_id}
    ↓
API Service
    ↓ gRPC
Report Service
    ↓
Redis / report_db
    ↓
Report
```

Analysis Orchestrator в чтении готового отчёта не участвует.

Прошлые отчёты доступны только авторизованному пользователю и читаются по схеме:

```text
Frontend
    ↓ HTTP
GET /api/v1/reports
    ↓
API Service
    ↓ gRPC
Report Service
    ↓
история отчётов текущего пользователя
```

История по конкретному репозиторию также требует пользовательского access JWT:

```text
GET /api/v1/repositories/{owner}/{name}/reports
```

## Скачивание PDF

```text
Авторизованный пользователь
    ↓ access JWT
GET /api/v1/reports/{report_id}/pdf
    ↓
API Service: проверка пользовательского JWT
    ↓ gRPC GetReportPdf(user_id, report_id)
Report Service: проверка владельца отчёта
    ↓
сохранённый Report → формирование PDF
    ↓ gRPC stream
API Service → HTTP application/pdf → браузер
```

Источником PDF является готовый immutable Report. Report Service читает его из cache или `report_db` с проверкой доступа. Для MVP PDF формируется по запросу и не требует нового сервиса или отдельного постоянного файлового хранилища.

Вход пользователя с существующим гостевым отчётом не предоставляет право скачать этот отчёт. Чтобы получить отчёт в истории аккаунта и PDF, пользователь запускает анализ в авторизованном режиме; reusable report может быть найден только среди отчётов его аккаунта.

Таким образом:

```text
Analysis Orchestrator → отвечает за состояние и завершение анализа.
Report Service        → отвечает за доступ к отчётам, историю аккаунта и PDF.
```

## Хранилище

Database:

```text
report_db
```

Таблица `reports`:

```text
reports
├── id
├── analysis_id
├── user_id (nullable)
├── guest_session_id (nullable)
├── repository_owner
├── repository_name
├── repository_url
├── repository_updated_at
├── repository_revision
├── analysis_window
├── collector_profile_version
├── overall_score
├── scoring_result_id
├── recommendation_result_id
├── generated_at
└── schema_version
```

Для `reports` действует ограничение:

```sql
CHECK ((user_id IS NOT NULL) <> (guest_session_id IS NOT NULL))
```

Заполнено ровно одно поле владельца. Оно соответствует владельцу `AnalysisRun`, полученному из команды `GenerateReport`. Для истории используется фильтр `user_id = current_user_id`; гостевые записи не попадают в результат.

Для быстрого чтения итогового отчёта допускается денормализация:

```text
report_category_scores
report_findings
report_recommendations
```

## Redis

Report Service использует Redis по cache-aside схеме.

Кэш отчёта:

```text
report:user:{user_id}:{report_id}
report:guest:{guest_session_id}:{report_id}
```

Тип и идентификатор владельца берутся из проверенного `Requester`. Кэш содержит владельца вместе с данными отчёта; Report Service сверяет его перед выдачей. Попадание в кэш не отменяет проверку доступа. PDF доступен только через пользовательский контекст и только для пользовательского отчёта.

Кэш списка отчётов пользователя:

```text
reports:{user_id}:list
```

При создании нового пользовательского отчёта список пользователя инвалидируется:

```text
DEL reports:{user_id}:list
```

Для гостевого отчёта кэш истории не создаётся. Любая конфигурация TTL кэша не продлевает срок гостевой сессии.

Source of truth остаётся в `report_db`.

---

# Передача данных между сервисами

Для межсервисного взаимодействия используется reference-based модель.

Kafka передаёт:

```text
analysis_id
snapshot_id
result_id
requester (идентификатор владельца в GenerateReport)
```

Полные доменные объекты и credentials через Kafka не передаются. Идентификатор владельца в `GenerateReport` берётся из `AnalysisRun` и нужен Report Service для сохранения принадлежности отчёта.

Сервис, которому нужны данные предыдущего этапа, получает их через gRPC у сервиса-владельца.

Схема:

```text
Analysis Service
    ↓
analysis_db
    ↓
Kafka:
AnalysisCompleted {
    analysis_id,
    analysis_snapshot_id
}
    ↓
Analysis Orchestrator
    ↓
Kafka:
CalculateScore {
    analysis_id,
    analysis_snapshot_id
}
    ↓
Scoring Service
    ↓ gRPC
Analysis Service.GetAnalysisSnapshot(...)
```

Далее:

```text
Scoring Service
    ↓
scoring_db
    ↓
Kafka:
ScoringCompleted {
    analysis_id,
    scoring_result_id
}
    ↓
Analysis Orchestrator
    ↓
Kafka:
GenerateRecommendations {
    analysis_id,
    analysis_snapshot_id,
    scoring_result_id
}
    ↓
Recommendation Service
    ├── gRPC → Analysis Service.GetAnalysisSnapshot(...)
    └── gRPC → Scoring Service.GetScoringResult(...)
```

Для формирования отчёта:

```text
Report Service
    ├── gRPC → GitHub API Service.GetGitHubDataSnapshot(...)
    ├── gRPC → Analysis Service.GetAnalysisSnapshot(...)
    ├── gRPC → Scoring Service.GetScoringResult(...)
    └── gRPC → Recommendation Service.GetRecommendationResult(...)
```

Таким образом:

- Kafka отвечает за запуск этапов и доставку событий;
- PostgreSQL хранит результат каждого сервиса;
- gRPC используется для синхронного чтения сохранённых результатов;
- сервисы не читают databases друг друга напрямую;
- Analysis Orchestrator не маппит и не пересылает полные доменные данные, а управляет только workflow и идентификаторами результатов.


# Observability

## Логи

Используется ELK Stack:

```text
Services
   ↓
structured JSON logs
   ↓
Logstash
   ↓
Elasticsearch
   ↓
Kibana
```

Рекомендуемые поля:

```text
timestamp
level
service
analysis_id
request_id
stage
message
error
```

`analysis_id` является основным correlation identifier для workflow анализа.

Пример:

```json
{
  "timestamp": "2026-09-30T10:00:00Z",
  "level": "error",
  "service": "scoring-service",
  "analysis_id": "A123",
  "stage": "SCORING",
  "message": "LLM request failed"
}
```

## Метрики

Используется:

```text
Prometheus
    ↓
Grafana
```

Основной подход:

```text
RED:
Rate
Errors
Duration
```

Примеры:

```text
http_requests_total
http_request_errors_total
http_request_duration_seconds

analyses_started_total
analyses_completed_total
analyses_failed_total
analysis_duration_seconds

kafka_consumer_lag
kafka_messages_processed_total
kafka_messages_failed_total
kafka_messages_dlq_total
service_retry_attempts_total

sandbox_active
sandbox_creation_duration_seconds
sandbox_failures_total

github_requests_total
github_request_errors_total
github_rate_limit_remaining

llm_requests_total
llm_request_errors_total
llm_request_duration_seconds
```

---

# MVP Deployment

Для MVP каждый сервис запускается в одном экземпляре:

```text
1x API Service
1x Auth Service
1x Analysis Orchestrator
1x GitHub API Service
1x Repository Sandbox Service
1x Analysis Service
1x Scoring Service
1x Recommendation Service
1x Report Service
```

Инфраструктура:

```text
1x PostgreSQL container
1x Redis
1x Kafka
1x SonarQube Server

1x Elasticsearch
1x Logstash
1x Kibana

1x Prometheus
1x Grafana
```

Repository Sandbox Service работает в одном экземпляре, но создаёт временный gVisor sandbox на конкретный анализ.

---

# Итоговый flow анализа

Перед запуском frontend использует пользовательский access JWT либо получает гостевую cookie через `POST /api/v1/auth/guest-session`. API Service формирует `Requester` из проверенных credentials.

```text
Гость или авторизованный пользователь
 ↓
Frontend
 ↓ HTTP + guest cookie / access JWT
API Service
 ↓ gRPC StartAnalysis(requester, repository_url)
Analysis Orchestrator

PREPARING
 ├── GitHub API Service:
 │      GET repository.updated_at
 │
 └── Report Service:
        FindReusableReport(requester, repository, updated_at)

если report актуален и принадлежит тому же владельцу
        ↓
return existing Report

иначе

COLLECTING
 ├── Kafka → GitHub API Service
 │            CollectGitHubData
 │
 └── Kafka → Repository Sandbox Service
              CollectRepositoryData
              ↓
              gVisor
              ↓
              git clone → local collector → SonarScanner
              сохранить RepositoryData
              Kafka: RepositoryDataCollected

GitHubDataCollected + RepositoryDataCollected
        ↓

ANALYZING
        ↓
Analysis Service
 ├── gRPC → GitHub API Service.GetGitHubDataSnapshot(...)
 ├── gRPC → Repository Sandbox Service
 │            GetRepositoryData(sandbox_collection_id)
 │            → stream Git/file facts + Sonar task
 ├── SonarQube API → task status, metrics, issues
 └── metrics + findings + observations + completeness
        ↓
AnalysisSnapshot → analysis_db
        ↓
gRPC: ReleaseSandbox
        ↓
Kafka: AnalysisCompleted(analysis_snapshot_id)

SCORING
        ↓
Scoring Service
 ├── gRPC → Analysis Service.GetAnalysisSnapshot(...)
 └── AI/LLM
        ↓
ScoringResult → scoring_db
        ↓
Kafka: ScoringCompleted(scoring_result_id)

RECOMMENDING
        ↓
Recommendation Service
 ├── gRPC → Analysis Service.GetAnalysisSnapshot(...)
 ├── gRPC → Scoring Service.GetScoringResult(...)
 └── AI/LLM
        ↓
RecommendationResult → recommendation_db
        ↓
Kafka: RecommendationsGenerated(recommendation_result_id)

REPORTING
        ↓
Kafka: GenerateReport(requester, snapshot/result IDs)
        ↓
Report Service
 ├── gRPC → GitHub API Service.GetGitHubDataSnapshot(...)
 ├── gRPC → Analysis Service.GetAnalysisSnapshot(...)
 ├── gRPC → Scoring Service.GetScoringResult(...)
 └── gRPC → Recommendation Service.GetRecommendationResult(...)
        ↓
immutable Report + user_id / guest_session_id → report_db
        ↓
Kafka: ReportGenerated(report_id)
        ↓
Analysis Orchestrator
        ↓
REPORTING → COMPLETED

Frontend polling:
GET /api/v1/analyses/{analysis_id} + credentials
        ↓
Orchestrator: проверка владельца AnalysisRun
        ↓
{
  status: completed,
  report_id: ...
}

Просмотр своего отчёта гостем или пользователем:

Frontend
   ↓ GET /api/v1/reports/{report_id} + credentials
API Service
   ↓ gRPC GetReport(requester, report_id)
Report Service: проверка владельца
   ↓
Redis / report_db
   ↓
Report

Только авторизованный пользователь:
GET /api/v1/reports → история аккаунта
GET /api/v1/repositories/{owner}/{name}/reports → история аккаунта по repository
GET /api/v1/reports/{report_id}/pdf → PDF своего пользовательского отчёта
```

Гостевые отчёты сохраняют своего первоначального владельца. Вход или регистрация не добавляют их в историю аккаунта и не предоставляют доступ к их PDF.
