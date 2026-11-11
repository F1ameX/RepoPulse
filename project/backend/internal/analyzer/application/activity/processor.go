// Package activity connects collected Git history, versioned scoring and the
// immutable snapshot/report publication performed by the worker.
package activity

import (
	"context"
	"fmt"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type Processor struct{ engine *scoring.Engine }

func NewProcessor(engine *scoring.Engine) *Processor { return &Processor{engine: engine} }

func (p *Processor) Process(ctx context.Context, a domain.Analysis, session worker.GitSession, advance func(string) error) (worker.Output, error) {
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	if err := advance("calculating"); err != nil {
		return worker.Output{}, err
	}
	git := session.Data()
	category, err := p.engine.ActivityFromGit(a, git)
	if err != nil {
		return worker.Output{}, err
	}
	facts := git.Activity(a.Window)
	collection := snapshot.Collection{}
	for i := range category.Metrics {
		input := category.Metrics[i].Input
		key := snapshot.Source{Name: "git", Scope: domain.ActivityHistoryScope}
		if input.ID == "ACT_DAYS_SINCE_LAST" && input.Status == "complete" && facts.Source.Status != "complete" {
			// A complete pinned walk may establish the last commit independently
			// of an interrupted fetch of other refs. Preserve that proof's scope.
			key.Scope = "activity_last_commit"
			source := facts.Source
			source.Scope, source.Status, source.Reason = key.Scope, "complete", nil
			one := 1
			source.Sample = domain.Sample{Collected: &one, Total: &one}
			collection.Sources = append(collection.Sources, source)
		}
		commits := facts.Commits
		if input.ID == "ACT_DAYS_SINCE_LAST" {
			commits = nil
			if facts.LastCommit != nil {
				commits = []domain.ActivityCommit{*facts.LastCommit}
			}
		}
		addEvidence(&input, a.Repository, commits)
		collection.Inputs = append(collection.Inputs, snapshot.Input{Value: input, Sources: []snapshot.Source{key}})
	}
	data, err := snapshot.Build(a, git, collection)
	if err != nil {
		return worker.Output{}, err
	}
	// Re-evaluate exactly the detached inputs that will be stored, including
	// any completeness constraints enforced while assembling the snapshot.
	activityInputs := append([]domain.MetricInput{}, data.Inputs[:3]...)
	category, err = p.engine.ActivityFromInputs(a.AssessmentAt, a.Window, a.Versions, activityInputs)
	if err != nil {
		return worker.Output{}, err
	}
	reason := &domain.Reason{Code: "ACTIVITY_ONLY", Message: "Доступен отчёт активности",
		CannotConclude: "Общая оценка по пяти категориям ещё не рассчитана"}
	fallback := "no_findings"
	report := domain.ReportData{OverallReason: reason, Summary: "Сохранён анализ активности основной ветки",
		Coverage: category.Coverage, Categories: []domain.Category{category}, Findings: []domain.Finding{}, Recommendations: []domain.Recommendation{},
		Explanation: domain.Explanation{Mode: "deterministic_fallback", FallbackReason: &fallback,
			CatalogVersion: a.Versions.RecommendationCatalog, SchemaVersion: a.Versions.ExplanationSchema, PromptVersion: a.Versions.Prompt}}
	report.Coverage.Total = 12
	report.Coverage.Unavailable += 9
	report.Coverage.Percentage = float64(report.Coverage.Calculated) * 100 / 12
	for _, spec := range []struct {
		id          string
		from, until int
	}{{"documentation", 3, 5}, {"testing", 5, 6}, {"security", 6, 9}, {"ci_cd", 9, 12}} {
		unknown := domain.Category{ID: spec.id, Status: "unavailable", Reason: reason,
			Coverage: domain.Coverage{Total: spec.until - spec.from, Unavailable: spec.until - spec.from}, Metrics: []domain.Metric{}}
		for _, input := range data.Inputs[spec.from:spec.until] {
			unknown.Metrics = append(unknown.Metrics, domain.Metric{Input: input, RuleVersion: a.Versions.Methodology,
				Explanation: "Входные данные проверки не получены"})
		}
		report.Categories = append(report.Categories, unknown)
	}
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	return worker.Output{Snapshot: data, Report: report}, nil
}

func addEvidence(input *domain.MetricInput, repository domain.RepositoryMetadata, commits []domain.ActivityCommit) {
	input.Evidence = []domain.Evidence{}
	for _, commit := range commits[:min(len(commits), 10)] {
		sha := commit.SHA
		url := repository.CanonicalURL + "/commit/" + sha
		input.Evidence = append(input.Evidence, domain.Evidence{ID: fmt.Sprintf("%s:%s", input.ID, sha), Source: "git", Kind: "commit",
			Description: "Коммит основной ветки с подтверждённой датой committer", Revision: &sha, URL: &url})
	}
	input.EvidenceLimited = len(commits) > len(input.Evidence)
	if input.Status == "complete" || input.Status == "not_applicable" {
		total := len(commits)
		input.EvidenceTotal = &total
	}
}
