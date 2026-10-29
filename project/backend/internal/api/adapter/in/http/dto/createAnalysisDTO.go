package dto

type CreateAnalysisRequest struct {
	RequestID     string `json:"request_id"`
	RepositoryURL string `json:"repository_url"`
}

type CreateAnalysisResponse struct {
	Outcome    string        `json:"outcome"`
	AnalysisID string        `json:"analysis_id"`
	ReportID   *string       `json:"report_id"`
	Status     string        `json:"status"`
	Repository RepositoryDTO `json:"repository"`
}
