// Package activity connects collected Git history and tree, versioned scoring
// and the immutable snapshot/report publication performed by the worker.
package activity

import (
	"context"
	"fmt"
	"slices"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/dependencies"
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
	collector := dependencies.Collector{}
	dependencyCollection, err := collector.Collect(ctx, a, session)
	if err != nil {
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
	if !slices.ContainsFunc(git.Sources, func(source domain.SourceState) bool {
		return source.Source == "git" && source.Scope == domain.ActivityHistoryScope
	}) {
		collection.Sources = append(collection.Sources, facts.Source)
	}
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
	treeKey := snapshot.Source{Name: "git", Scope: "tree"}
	treeIndex := slices.IndexFunc(git.Sources, func(source domain.SourceState) bool {
		return source.Source == treeKey.Name && source.Scope == treeKey.Scope
	})
	tree := domain.SourceState{Source: treeKey.Name, Scope: treeKey.Scope, Status: "unavailable", Reason: &domain.Reason{
		Code: "GIT_TREE_UNAVAILABLE", Message: "Дерево основной ветки не получено",
		CannotConclude: "Нельзя проверить наличие документации и признаков тестов",
	}}
	if treeIndex >= 0 {
		tree = git.Sources[treeIndex]
	} else {
		collection.Sources = append(collection.Sources, tree)
	}
	for _, input := range p.engine.TreeInputs(git, tree) {
		collection.Inputs = append(collection.Inputs, snapshot.Input{Value: input, Sources: []snapshot.Source{treeKey}})
	}
	data, err := snapshot.Build(a, git, collection, dependencyCollection)
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
	treeCategories, err := p.engine.TreeCategories(a.Versions, data.Inputs)
	if err != nil {
		return worker.Output{}, err
	}
	overallReason := &domain.Reason{
		Code: "INCOMPLETE_CATEGORIES", Message: "Общая оценка недоступна",
		CannotConclude: "Общая оценка по пяти категориям ещё не рассчитана",
	}
	fallback := "no_findings"
	report := domain.ReportData{
		OverallReason: overallReason, Summary: "Сохранён анализ активности, документации и файловых признаков тестов основной ветки",
		Categories: append([]domain.Category{category}, treeCategories...), Findings: []domain.Finding{}, Recommendations: []domain.Recommendation{},
		Explanation: domain.Explanation{
			Mode: "deterministic_fallback", FallbackReason: &fallback,
			CatalogVersion: a.Versions.RecommendationCatalog, SchemaVersion: a.Versions.ExplanationSchema, PromptVersion: a.Versions.Prompt,
		},
	}
	pending := &domain.Reason{
		Code: "INPUT_NOT_COLLECTED", Message: "Входные данные проверки не получены",
		CannotConclude: "Оценка категории недоступна",
	}
	for _, spec := range []struct {
		id          string
		from, until int
	}{{"security", 6, 9}, {"ci_cd", 9, 12}} {
		unknown := domain.Category{
			ID: spec.id, Status: "unavailable", Reason: pending,
			Coverage: domain.Coverage{Total: spec.until - spec.from, Unavailable: spec.until - spec.from}, Metrics: []domain.Metric{},
		}
		for _, input := range data.Inputs[spec.from:spec.until] {
			unknown.Metrics = append(unknown.Metrics, domain.Metric{
				Input: input, RuleVersion: a.Versions.Methodology,
				Explanation: "Входные данные проверки не получены",
			})
		}
		report.Categories = append(report.Categories, unknown)
	}
	for _, item := range report.Categories {
		report.Coverage.Total += item.Coverage.Total
		report.Coverage.Calculated += item.Coverage.Calculated
		report.Coverage.Partial += item.Coverage.Partial
		report.Coverage.Unavailable += item.Coverage.Unavailable
		report.Coverage.NotApplicable += item.Coverage.NotApplicable
	}
	report.Coverage.Percentage = float64(report.Coverage.Calculated) * 100 / float64(report.Coverage.Total)
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
		input.Evidence = append(input.Evidence, domain.Evidence{
			ID: fmt.Sprintf("%s:%s", input.ID, sha), Source: "git", Kind: "commit",
			Description: "Коммит основной ветки с подтверждённой датой committer", Revision: &sha, URL: &url,
		})
	}
	input.EvidenceLimited = len(commits) > len(input.Evidence)
	if input.Status == "complete" || input.Status == "not_applicable" {
		total := len(commits)
		input.EvidenceTotal = &total
	}
}
