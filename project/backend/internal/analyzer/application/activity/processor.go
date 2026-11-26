// Package activity connects collected Git history and tree, versioned scoring
// and the immutable snapshot/report publication performed by the worker.
package activity

import (
	"context"
	"fmt"
	"slices"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/ci"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/dependencies"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// CICollector supplies normalized GitHub facts and the coverage of each scope.
// Upstream refusals and incomplete responses are represented by SourceState.
type CICollector interface {
	CollectCI(context.Context, domain.Analysis) (ci.ExternalData, error)
}

type Processor struct {
	engine      *scoring.Engine
	ciCollector CICollector
	osv         snapshot.OSVQuerier
}

func NewProcessor(engine *scoring.Engine) *Processor { return &Processor{engine: engine} }

func NewProcessorWithCI(engine *scoring.Engine, collector CICollector) *Processor {
	return &Processor{engine: engine, ciCollector: collector}
}

func NewProcessorWithSources(engine *scoring.Engine, collector CICollector, osv snapshot.OSVQuerier) *Processor {
	return &Processor{engine: engine, ciCollector: collector, osv: osv}
}

func (p *Processor) Process(ctx context.Context, a domain.Analysis, session worker.GitSession, advance func(string) error) (worker.Output, error) {
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	collector := dependencies.Collector{}
	dependencyCollection, err := collector.Collect(ctx, a, session)
	if err != nil {
		return worker.Output{}, err
	}
	git := session.Data()
	treeKey := snapshot.Source{Name: "git", Scope: domain.GitTreeScope}
	treeIndex := slices.IndexFunc(git.Sources, func(source domain.SourceState) bool {
		return source.Source == treeKey.Name && source.Scope == treeKey.Scope
	})
	tree := domain.SourceState{Source: treeKey.Name, Scope: treeKey.Scope, Status: "unavailable", Reason: &domain.Reason{
		Code: "GIT_TREE_UNAVAILABLE", Message: "Дерево основной ветки не получено",
		CannotConclude: "Нельзя проверить документацию, признаки тестов и конфигурацию CI",
	}}
	if treeIndex >= 0 {
		tree = git.Sources[treeIndex]
	}
	sast := ci.SASTInput(ctx, session, tree)
	var ciData ci.ExternalData
	if p.ciCollector != nil || p.osv != nil {
		if err := advance("collecting_external"); err != nil {
			return worker.Output{}, err
		}
	}
	if p.ciCollector != nil {
		ciData, err = p.ciCollector.CollectCI(ctx, a)
		if err != nil {
			return worker.Output{}, err
		}
	}
	var osvCollection snapshot.Collection
	if p.osv != nil {
		osvCollection, err = snapshot.CollectOSV(ctx, p.osv, []snapshot.Collection{dependencyCollection})
		if err != nil {
			return worker.Output{}, err
		}
	}
	if err := advance("calculating"); err != nil {
		return worker.Output{}, err
	}
	category, err := p.engine.ActivityFromGit(a, git)
	if err != nil {
		return worker.Output{}, err
	}
	facts := git.Activity(a.Window)
	collection := snapshot.Collection{}
	if treeIndex < 0 {
		collection.Sources = append(collection.Sources, tree)
	}
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
	for _, input := range p.engine.TreeInputs(git, tree) {
		collection.Inputs = append(collection.Inputs, snapshot.Input{Value: input, Sources: []snapshot.Source{treeKey}})
	}
	data, err := snapshot.Build(a, git, collection, dependencyCollection, osvCollection, ci.Inputs(a, git, tree, ciData), sast)
	if err != nil {
		return worker.Output{}, err
	}
	// Re-evaluate exactly the detached inputs that will be stored, including
	// any completeness constraints enforced while assembling the snapshot.
	p.engine.PrepareVulnerabilityRisk(&data)
	report, err := p.engine.Calculate(ctx, a, data)
	if err != nil {
		return worker.Output{}, err
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
