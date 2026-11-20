// Package ci builds the reproducible inputs for the CI/CD category.
package ci

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Attempt describes one Actions attempt. CreatedAt is the run creation time
// shared by all its attempts. CompletedAt is zero while an attempt is unfinished.
type Attempt struct {
	RunID       uint64
	Number      int
	CreatedAt   time.Time
	CompletedAt time.Time
	Conclusion  string
}

// ExternalData is supplied by the GitHub collector. RequiredStatusChecks counts
// checks enforced by applicable default branch rules, excluding review rules.
// A complete Rules source confirms the entire applicable set was inspected.
type ExternalData struct {
	facts                *domain.GitHubCI
	Actions              domain.SourceState
	Attempts             []Attempt
	Rules                domain.SourceState
	RequiredStatusChecks int
}

type Collector struct {
	Reader snapshot.CIReader
}

func (collector Collector) CollectCI(ctx context.Context, analysis domain.Analysis) (ExternalData, error) {
	facts, sources := collector.Reader.CollectCI(ctx, analysis.Repository, analysis.Window)
	if err := ctx.Err(); err != nil {
		return ExternalData{}, err
	}
	data := ExternalData{facts: &facts}
	for _, source := range sources {
		switch source.Scope {
		case domain.ActionsScope:
			data.Actions = source
		case domain.BranchRulesScope:
			data.Rules = source
		}
	}
	for _, run := range facts.Attempts {
		id, err := strconv.ParseUint(run.RunID, 10, 64)
		if err != nil {
			id = 0
		}
		attempt := Attempt{RunID: id, Number: run.Attempt, CreatedAt: run.CreatedAt, Conclusion: run.Conclusion}
		if run.Status == "completed" {
			attempt.CompletedAt = run.UpdatedAt
		}
		data.Attempts = append(data.Attempts, attempt)
	}
	if facts.ClassicRequiredChecks != nil {
		data.RequiredStatusChecks = *facts.ClassicRequiredChecks
	}
	for _, rule := range facts.Rules {
		data.RequiredStatusChecks += rule.RequiredChecks
	}
	return data, nil
}

func Inputs(a domain.Analysis, git domain.GitData, tree domain.SourceState, external ExternalData) snapshot.Collection {
	presence, actionsConfig := presenceInput(git, tree)
	actions := githubSource(external.Actions, "actions", "CI_ACTIONS_UNAVAILABLE",
		"История GitHub Actions недоступна", "Нельзя подтвердить долю успешных запусков")
	rules := githubSource(external.Rules, "branch_rules", "CI_RULES_UNAVAILABLE",
		"Полный набор правил основной ветки недоступен", "Нельзя подтвердить наличие обязательных CI-проверок")

	success := metricInput("CI_SUCCESS_RATE", "ratio", actions)
	success.Window = &a.Window
	successSources := []snapshot.Source{{Name: "github", Scope: "actions"}}
	if tree.Status == "complete" && !actionsConfig {
		success.Status = "not_applicable"
		success.Reason = &domain.Reason{Code: "CI_NOT_CONFIGURED", Message: "В дереве нет распознанной конфигурации CI",
			CannotConclude: "Доля успешных запусков неприменима"}
		if len(presence.Evidence) > 0 {
			success.Reason = &domain.Reason{Code: "CI_EXTERNAL", Message: "Распознана только конфигурация внешнего CI",
				CannotConclude: "История запусков доступна в MVP только для GitHub Actions"}
		}
		success.ObservedAt = tree.ObtainedAt
		successSources = []snapshot.Source{{Name: "git", Scope: domain.GitTreeScope}}
	} else {
		actions, success = successInput(a, actions, external.Attempts)
	}

	required := metricInput("CI_REQUIRED_CHECKS", "bool", rules)
	required.Sample = rules.Sample
	if rules.Status == "complete" {
		required.RawValue = scalar(strconv.FormatBool(external.RequiredStatusChecks > 0))
	}
	if rules.Status == "complete" || rules.Status == "partial" && external.RequiredStatusChecks > 0 {
		description := "В полном наборе применимых правил нет обязательных CI-проверок"
		if external.RequiredStatusChecks > 0 {
			description = fmt.Sprintf("Подтверждено обязательных CI-проверок: %d", external.RequiredStatusChecks)
		}
		required.Evidence = append(required.Evidence, domain.Evidence{
			ID: "CI_REQUIRED_CHECKS:branch_rules", Source: "github", Kind: "branch_rule", Description: description,
		})
		if rules.Status == "complete" {
			required.EvidenceTotal = new(len(required.Evidence))
		}
	}
	var facts *domain.GitHubCI
	if external.facts != nil {
		facts = external.facts.Clone()
		facts.CollectConfigurations(git)
	}
	return snapshot.Collection{
		GitHubCI: facts,
		Sources:  []domain.SourceState{actions, rules},
		Inputs: []snapshot.Input{
			{Value: presence, Sources: []snapshot.Source{{Name: "git", Scope: domain.GitTreeScope}}},
			{Value: success, Sources: successSources},
			{Value: required, Sources: []snapshot.Source{{Name: "github", Scope: "branch_rules"}}},
		},
	}
}

func presenceInput(git domain.GitData, tree domain.SourceState) (domain.MetricInput, bool) {
	input := metricInput("CI_PRESENT", "bool", tree)
	input.Sample = tree.Sample
	if tree.Status != "complete" && tree.Status != "partial" {
		return input, false
	}
	var paths []string
	actions := false
	for _, file := range git.Files {
		if (file.Mode != "100644" && file.Mode != "100755") || !domain.ValidEvidencePath(file.Path) {
			continue
		}
		dir, name := path.Split(file.Path)
		workflow := dir == ".github/workflows/" && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml"))
		switch file.Path {
		case ".gitlab-ci.yml", "Jenkinsfile", ".circleci/config.yml", "azure-pipelines.yml":
		default:
			if !workflow {
				continue
			}
		}
		actions = actions || workflow
		paths = append(paths, file.Path)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	for _, name := range paths[:min(len(paths), evidenceLimit)] {
		input.Evidence = append(input.Evidence, domain.Evidence{
			ID: "CI_PRESENT:" + name, Source: "git", Kind: "file", Path: &name, Revision: git.Repository.CommitSHA,
			Description: "Распознанный файл конфигурации CI в дереве основной ветки",
		})
	}
	input.EvidenceLimited = len(paths) > len(input.Evidence)
	if tree.Status == "complete" {
		input.RawValue = scalar(strconv.FormatBool(len(paths) > 0))
		input.EvidenceTotal = new(len(paths))
	}
	return input, actions
}

func githubSource(source domain.SourceState, scope, code, message, cannotConclude string) domain.SourceState {
	source.Source, source.Scope = "github", scope
	if source.Status == "" {
		source.Status = "unavailable"
		source.Reason = &domain.Reason{Code: code, Message: message, CannotConclude: cannotConclude}
	}
	return source
}

func metricInput(id, unit string, source domain.SourceState) domain.MetricInput {
	return domain.MetricInput{
		ID: id, Unit: unit, Status: source.Status, Reason: source.Reason, ObservedAt: source.ObtainedAt,
		Series: []int{}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{},
	}
}

func scalar(value string) domain.Scalar {
	var raw domain.Scalar
	_ = raw.UnmarshalJSON([]byte(value))
	return raw
}

const evidenceLimit = 10
