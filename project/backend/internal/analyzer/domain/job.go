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

// Failure only accepts the public terminal codes, never raw process/provider errors.
func Failure(code, traceID string) AnalysisFailure {
	message := "Не удалось получить пригодный результат анализа"
	switch code {
	case "BUDGET_EXCEEDED":
		message = "Превышен допустимый бюджет анализа"
	case "REPORT_SAVE_FAILED":
		message = "Не удалось сохранить результат анализа"
	default:
		code = "ANALYSIS_FAILED"
	}
	return AnalysisFailure{Code: code, Message: message, TraceID: traceID, FieldErrors: []FailureField{}}
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
