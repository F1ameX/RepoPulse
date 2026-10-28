# Карта полей по сервисам

Для каждого поля описаны тип и ограничения JSON, тип и номер protobuf,
обязательность, допустимость null, смысл и хранение. Общие имена полей — snake_case.
HTTP DTO и gRPC-сообщения реализуются отдельно; 0, false и отсутствие значения различаются.

- [common](fields/common.md) — `contracts/common/v1/schemas.json`
- [auth](fields/auth.md) — `contracts/auth/v1/schemas.json`
- [analyzer](fields/analyzer.md) — `contracts/analyzer/v1/schemas.json`
- [report](fields/report.md) — `contracts/report/v1/schemas.json`

[Gateway HTTP](../contracts/gateway/v1/openapi.json) ссылается на эти модели.

[HTTP операции и параметры](../contracts/gateway/README.md).
[Движение данных между компонентами](architecture/data-flow.md).

[Ошибки HTTP/gRPC](errors.md).

`ApiError.code` включает `AUTH_UNAVAILABLE` (HTTP 503), общий для подключённых
HTTP-операций. OpenAPI использует общий enum по ссылке;
[пример ответа](../contracts/examples/auth-unavailable.json).
