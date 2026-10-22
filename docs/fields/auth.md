# Поля auth

JSON Schema: [schemas.json](../../contracts/auth/v1/schemas.json).
Protobuf: [auth.proto](../../contracts/auth/v1/auth.proto).

Здесь описаны поля моделей, а не автоматическое преобразование protobuf в JSON.
HTTP публикует только модели, на которые ссылается [OpenAPI Gateway](../../contracts/gateway/v1/openapi.json).
Внутренние контексты, токены, команды и Snapshot не принимаются от браузера.
В protobuf optional хранит наличие скаляра; поля-сообщения имеют собственный признак наличия.
В JSON обязательное поле присутствует даже при null; неизвестные списки и пустые списки различаются по состоянию источника.

## gRPC операции

| Метод | Запрос | Ответ |
|---|---|---|
| `Register` | `Credentials` | `RegistrationResult` |
| `Login` | `Credentials` | `IssuedSession` |
| `CreateGuestSession` | `repopulse.common.v1.Empty` | `IssuedSession` |
| `ResolveContext` | `SessionTokens` | `repopulse.common.v1.VerifiedContext` |
| `Logout` | `LogoutCommand` | `repopulse.common.v1.Empty` |

## Credentials

Нормализация и проверки в указанном FR-27 порядке обязательны на клиенте и сервере.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `Credentials.login` | `string` | `string` #1 | да / нет | FR-27: trim только U+0020, 3–32 ASCII, первая буква, далее буквы/цифры/_. Lowercase для хранения. | — |
| `Credentials.password` | `string (password)` | `string` #2 | да / нет | FR-27: NFC, 15–128 Unicode code points; L/M/N/P/S и U+0020. Регистр и внешние пробелы сохраняются. Валидировать после NFC; не логировать. | — |

## RegistrationResult

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `RegistrationResult.login` | `string` | `string` #1 | да / нет | Нормализованный логин. | — |
| `RegistrationResult.message` | `string` | `string` #2 | да / нет | Аккаунт создан. Войдите в систему | — |

## SessionInfo

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `SessionInfo.kind` | `string; account, guest` | `string` #1 | да / нет | Тип действующей сессии: account либо guest. | auth.sessions / auth.principals.login (без session id и token) → kind |
| `SessionInfo.login` | `string / null` | `optional string` #2 | да / да | Только нормализованный login аккаунта; guest=null. | auth.sessions / auth.principals.login (без session id и token) → login |
| `SessionInfo.expires_at` | `string (date-time)` | `google.protobuf.Timestamp` #3 | да / нет | Account: ровно 7 суток без продления; guest: не более 24 ч и браузерная сессия. | auth.sessions / auth.principals.login (без session id и token) → expires_at |

## SessionContext

Оба поля обязательны, отсутствие представляется null. Проверка cookie перед анализом.

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `SessionContext.account` | `SessionInfo / null` | `SessionInfo` #1 | да / да | Сведения о текущей сессии аккаунта; null, если действующего аккаунта нет. | — |
| `SessionContext.guest` | `SessionInfo / null` | `SessionInfo` #2 | да / да | Сведения об исходной гостевой сессии; null, если она отсутствует или истекла. | — |

## SessionTokens

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `SessionTokens.account_token` | `string / null` | `optional string` #1 | да / да | Только внутренний Auth RPC, секрет. | — |
| `SessionTokens.guest_token` | `string / null` | `optional string` #2 | да / да | Только внутренний Auth RPC, секрет. | — |

## IssuedSession

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `IssuedSession.token` | `string` | `string` #1 | да / нет | Только внутренний ответ Auth, устанавливается Gateway в HttpOnly cookie. | — |
| `IssuedSession.session` | `SessionInfo` | `SessionInfo` #2 | да / нет | Публичные сведения о выданной сессии без её токена. | — |

## LogoutCommand

| Поле | JSON: тип и значения | protobuf: тип и номер | Обязательное в JSON / null | Описание | Хранение |
|---|---|---|---|---|---|
| `LogoutCommand.account_token` | `string` | `string` #1 | да / нет | Отозвать только текущую сессию, повтор безопасен. | — |

