package domain

import (
	"slices"
	"strings"
	"time"
)

const (
	ActionsScope     = "actions"
	BranchRulesScope = "branch_rules"
)

// GitHubCI contains the normalized inputs for CI analysis. It is persisted in
// Analyzer storage, separately from the versioned Snapshot wire fields.
type GitHubCI struct {
	Attempts           []ActionsAttempt `json:"attempts"`
	Rules              []BranchRule     `json:"rules"`
	ConfigPaths        []string         `json:"config_paths"`
	ConfigTreeComplete bool             `json:"config_tree_complete"`
	// Nil means classic branch protection could not be established.
	ClassicRequiredChecks *int       `json:"classic_required_checks"`
	ClassicObtainedAt     *time.Time `json:"classic_obtained_at"`
}

type ActionsAttempt struct {
	RunID      string     `json:"run_id"`
	Attempt    int        `json:"attempt"`
	HeadSHA    string     `json:"head_sha"`
	Status     string     `json:"status"`
	Conclusion string     `json:"conclusion"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ObtainedAt time.Time  `json:"obtained_at"`
}

type BranchRule struct {
	RulesetID      string    `json:"ruleset_id"`
	Type           string    `json:"type"`
	RequiredChecks int       `json:"required_checks"`
	ObtainedAt     time.Time `json:"obtained_at"`
}

func (ci GitHubCI) Clone() *GitHubCI {
	ci.Attempts = slices.Clone(ci.Attempts)
	ci.Rules = slices.Clone(ci.Rules)
	ci.ConfigPaths = slices.Clone(ci.ConfigPaths)
	for i := range ci.Attempts {
		if at := ci.Attempts[i].StartedAt; at != nil {
			copy := *at
			ci.Attempts[i].StartedAt = &copy
		}
	}
	if count := ci.ClassicRequiredChecks; count != nil {
		copy := *count
		ci.ClassicRequiredChecks = &copy
	}
	if at := ci.ClassicObtainedAt; at != nil {
		copy := *at
		ci.ClassicObtainedAt = &copy
	}
	return &ci
}

// LatestCompleted returns the latest observed completed attempt in the
// window, ordered by the GitHub update timestamp. The caller uses the Actions
// source state to determine whether the observation covers the whole window.
func (ci GitHubCI) LatestCompleted(window Window) (ActionsAttempt, bool) {
	var latest ActionsAttempt
	for _, attempt := range ci.CompletedAttempts(window) {
		if latest.RunID == "" || attempt.UpdatedAt.After(latest.UpdatedAt) ||
			attempt.UpdatedAt.Equal(latest.UpdatedAt) && (attempt.RunID > latest.RunID ||
				attempt.RunID == latest.RunID && attempt.Attempt > latest.Attempt) {
			latest = attempt
		}
	}
	return latest, latest.RunID != ""
}

// CompletedAttempts keeps the latest completed attempt per run inside [from, until).
func (ci GitHubCI) CompletedAttempts(window Window) []ActionsAttempt {
	byRun := map[string]ActionsAttempt{}
	for _, attempt := range ci.Attempts {
		if attempt.Status == "completed" && !attempt.UpdatedAt.Before(window.From) && attempt.UpdatedAt.Before(window.Until) &&
			attempt.Attempt > byRun[attempt.RunID].Attempt {
			byRun[attempt.RunID] = attempt
		}
	}
	attempts := make([]ActionsAttempt, 0, len(byRun))
	for _, attempt := range byRun {
		attempts = append(attempts, attempt)
	}
	slices.SortFunc(attempts, func(a, b ActionsAttempt) int { return strings.Compare(a.RunID, b.RunID) })
	return attempts
}

// CollectConfigurations takes configuration evidence from the pinned tree.
// Runs can be present or absent independently of that configuration.
func (ci *GitHubCI) CollectConfigurations(git GitData) {
	ci.ConfigPaths = []string{}
	for _, file := range git.Files {
		if (file.Mode == "100644" || file.Mode == "100755") && ciConfigPath(file.Path) {
			ci.ConfigPaths = append(ci.ConfigPaths, file.Path)
		}
	}
	slices.Sort(ci.ConfigPaths)
	ci.ConfigPaths = slices.Compact(ci.ConfigPaths)
	for _, source := range git.Sources {
		if source.Source == "git" && source.Scope == GitTreeScope {
			ci.ConfigTreeComplete = source.Status == "complete"
			break
		}
	}
}

func ciConfigPath(path string) bool {
	if !ValidEvidencePath(path) {
		return false
	}
	switch path {
	case ".gitlab-ci.yml", "Jenkinsfile", ".circleci/config.yml", "azure-pipelines.yml":
		return true
	}
	name, ok := strings.CutPrefix(path, ".github/workflows/")
	return ok && !strings.Contains(name, "/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
}

func (ci GitHubCI) ConfigurationPresent() *bool {
	if !ci.ConfigTreeComplete {
		return nil
	}
	present := len(ci.ConfigPaths) > 0
	return &present
}

// RunsPresent describes evidence of runs in the window. An empty Actions
// listing alone does not establish the absence of a CI configuration.
func (ci GitHubCI) RunsPresent(source SourceState) *bool {
	present := len(ci.Attempts) > 0
	if present || source.Status == "complete" {
		return &present
	}
	return nil
}

// RequiredChecks requires a complete read of both rulesets and classic protection.
func (ci GitHubCI) RequiredChecks(source SourceState) *bool {
	if source.Status != "complete" || ci.ClassicRequiredChecks == nil {
		return nil
	}
	confirmed := *ci.ClassicRequiredChecks > 0
	for _, rule := range ci.Rules {
		confirmed = confirmed || rule.RequiredChecks > 0
	}
	return &confirmed
}

func (attempt ActionsAttempt) Valid() bool {
	if !githubNumber(attempt.RunID) || attempt.Attempt <= 0 || !shaPattern.MatchString(attempt.HeadSHA) ||
		attempt.CreatedAt.IsZero() || attempt.UpdatedAt.Before(attempt.CreatedAt) || attempt.ObtainedAt.IsZero() ||
		attempt.StartedAt != nil && (attempt.StartedAt.Before(attempt.CreatedAt) || attempt.StartedAt.After(attempt.UpdatedAt)) {
		return false
	}
	switch attempt.Status {
	case "completed":
		switch attempt.Conclusion {
		case "success", "failure", "neutral", "cancelled", "skipped", "timed_out", "action_required", "stale", "startup_failure":
			return true
		}
	case "requested", "queued", "pending", "waiting", "in_progress":
		return attempt.Conclusion == ""
	}
	return false
}

func (rule BranchRule) Valid() bool {
	if !githubNumber(rule.RulesetID) || rule.Type == "" || len(rule.Type) > 64 || rule.RequiredChecks < 0 || rule.ObtainedAt.IsZero() {
		return false
	}
	for _, ch := range rule.Type {
		if (ch < 'a' || ch > 'z') && ch != '_' {
			return false
		}
	}
	return true
}

func githubNumber(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (ci GitHubCI) validate(sources []SourceState) error {
	actions, rules := false, false
	for _, source := range sources {
		if source.Source == "github" {
			actions = actions || source.Scope == ActionsScope
			rules = rules || source.Scope == BranchRulesScope
		}
	}
	if !actions || !rules || ci.Attempts == nil || ci.Rules == nil ||
		ci.ClassicRequiredChecks != nil && (*ci.ClassicRequiredChecks < 0 || ci.ClassicObtainedAt == nil || ci.ClassicObtainedAt.IsZero()) {
		return invalid("github ci inputs")
	}
	type key struct {
		id      string
		attempt int
	}
	seen := make(map[key]bool, len(ci.Attempts))
	for _, attempt := range ci.Attempts {
		k := key{attempt.RunID, attempt.Attempt}
		if !attempt.Valid() || seen[k] {
			return invalid("github actions attempt")
		}
		seen[k] = true
	}
	for _, rule := range ci.Rules {
		if !rule.Valid() {
			return invalid("github branch rule")
		}
	}
	for _, path := range ci.ConfigPaths {
		if !ciConfigPath(path) {
			return invalid("ci configuration path")
		}
	}
	return nil
}
