package domain

import "time"

type AnalysisFailure struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	TraceID     string         `json:"trace_id"`
	FieldErrors []FailureField `json:"field_errors"`
}
type FailureField struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

const FailureNoUsefulData = "NO_USEFUL_DATA"

const (
	gitSizeLimitMessage = "Репозиторий превышает ограничения анализа"
	noUsefulDataMessage = "Не удалось получить данные для анализа"
)

func IsGitLimit(reason string) bool {
	return reason == "GIT_TIME_LIMIT" || isGitSizeLimit(reason)
}

func isGitSizeLimit(reason string) bool {
	switch reason {
	case "GIT_SIZE_LIMIT", "GIT_PATH_LIMIT", "GIT_COMMIT_LIMIT", "GIT_OUTPUT_LIMIT":
		return true
	default:
		return false
	}
}

// Failure translates internal reasons to the public error codes and fixed messages.
func Failure(code, traceID string) AnalysisFailure {
	message := "Не удалось завершить анализ"
	switch {
	case code == "BUDGET_EXCEEDED" || code == "GIT_TIME_LIMIT":
		code = "BUDGET_EXCEEDED"
		message = "Превышено время анализа"
	case isGitSizeLimit(code):
		code, message = "ANALYSIS_FAILED", gitSizeLimitMessage
	case code == FailureNoUsefulData:
		code, message = "ANALYSIS_FAILED", noUsefulDataMessage
	case code == "REPORT_SAVE_FAILED":
		message = "Не удалось сохранить результат анализа"
	default:
		code = "ANALYSIS_FAILED"
	}
	return AnalysisFailure{Code: code, Message: message, TraceID: traceID, FieldErrors: []FailureField{}}
}

// Sanitized reconstructs the error before storing it or sending it to a client.
// It also translates legacy rows that stored a Git reason as the public code.
func (f AnalysisFailure) Sanitized() AnalysisFailure {
	safe := Failure(f.Code, f.TraceID)
	if f.Code == "ANALYSIS_FAILED" {
		switch f.Message {
		case gitSizeLimitMessage, noUsefulDataMessage:
			safe.Message = f.Message
		}
	}
	return safe
}

func StageOrder(stage string) int {
	switch stage {
	case "collecting_git":
		return 1
	case "collecting_external":
		return 2
	case "calculating":
		return 3
	case "explaining":
		return 4
	case "saving":
		return 5
	default:
		return 0
	}
}

func CanAdvance(a Analysis, stage string, now time.Time) bool {
	if a.Status != "running" || a.DeadlineAt == nil || !now.Before(*a.DeadlineAt) || StageOrder(stage) == 0 {
		return false
	}
	return a.Stage == nil || StageOrder(stage) >= StageOrder(*a.Stage)
}
