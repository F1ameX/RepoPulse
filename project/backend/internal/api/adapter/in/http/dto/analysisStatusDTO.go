package dto

import "time"

type AnalysisStatusDTO struct {
	AnalysisID  string        `json:"analysis_id"`
	Repository  RepositoryDTO `json:"repository"`
	Status      string        `json:"status"`
	Stage       *string       `json:"stage"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at"`
	ReportID    *string       `json:"report_id"`
	Error       *APIError     `json:"error"`
}
