package scoring

import "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"

// ActivityFromGit calculates the activity category and its snapshot-ready
// MetricInputs using the metadata fixed at acceptance and the collected Git facts.
func (e *Engine) ActivityFromGit(analysis domain.Analysis, data domain.GitData) (domain.Category, error) {
	collected := data.Activity(analysis.Window)
	history := collected.Source
	// Keep the existing scoring input scope. The published collector source
	// retains activity_history, independently of the all-refs history scope.
	history.Scope = "history"
	facts := ActivityData{
		History:       history,
		WindowCommits: len(collected.Commits),
		Periods:       collected.Periods,
	}
	if last := collected.LastCommit; last != nil {
		facts.LastCommitAt = &last.CommittedAt
		facts.LastCommitConfirmed = history.Status != "complete"
	}
	return e.Activity(analysis.AssessmentAt, collected.Window, analysis.Versions, facts)
}
