package dto

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	TraceID     string       `json:"trace_id"`
	FieldErrors []FieldError `json:"field_errors"`
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
