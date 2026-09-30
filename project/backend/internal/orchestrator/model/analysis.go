// Package model holds Analysis Orchestrator domain types. Platform DTOs stay in adapters.
package model

type AnalysisStatus string

const (
	StatusQueued    AnalysisStatus = "queued"
	StatusRunning   AnalysisStatus = "running"
	StatusCompleted AnalysisStatus = "completed"
	StatusFailed    AnalysisStatus = "failed"
)

type Stage string

const (
	StagePreparing    Stage = "PREPARING"
	StageCollecting   Stage = "COLLECTING"
	StageAnalyzing    Stage = "ANALYZING"
	StageScoring      Stage = "SCORING"
	StageRecommending Stage = "RECOMMENDING"
	StageReporting    Stage = "REPORTING"
)

// Repository identifies a repository without depending on a platform API DTO.
type Repository struct {
	URL   string
	Owner string
	Name  string
}
