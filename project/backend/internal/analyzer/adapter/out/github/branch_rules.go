package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type requiredCheck struct {
	Context string `json:"context"`
}

type ruleKey struct{ id, kind string }

func readRuleKey(raw []byte) ruleKey {
	var item struct {
		ID   json.Number `json:"ruleset_id"`
		Type string      `json:"type"`
	}
	if json.Unmarshal(raw, &item) != nil || !validID(item.ID.String()) || strings.TrimSpace(item.Type) == "" {
		return ruleKey{}
	}
	return ruleKey{item.ID.String(), item.Type}
}

// Compare rule contents before reducing them to a count. Two definitions of
// the same rule can require different checks even when the counts match.
func ruleParameters(raw []byte) ([]byte, bool) {
	var item struct{ Parameters any }
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&item) != nil {
		return nil, false
	}
	if params, ok := item.Parameters.(map[string]any); ok {
		for _, key := range []string{"required_status_checks", "workflows"} {
			if list, ok := params[key].([]any); ok {
				entries := make([]json.RawMessage, 0, len(list))
				for _, value := range list {
					encoded, err := json.Marshal(value)
					if err != nil {
						return nil, false
					}
					entries = append(entries, encoded)
				}
				sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i], entries[j]) < 0 })
				params[key] = entries
			}
		}
	}
	normalized, err := json.Marshal(item.Parameters)
	return normalized, err == nil
}

func countChecks(checks []requiredCheck) (int, bool) {
	for _, check := range checks {
		if strings.TrimSpace(check.Context) == "" {
			return 0, false
		}
	}
	return len(checks), true
}

func decodeRule(raw []byte, at time.Time) (domain.BranchRule, bool) {
	var item struct {
		Type       string      `json:"type"`
		RulesetID  json.Number `json:"ruleset_id"`
		Parameters struct {
			Checks    []requiredCheck `json:"required_status_checks"`
			Workflows []struct {
				RepositoryID json.Number `json:"repository_id"`
				Path         string      `json:"path"`
			} `json:"workflows"`
		} `json:"parameters"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return domain.BranchRule{}, false
	}
	rule := domain.BranchRule{RulesetID: item.RulesetID.String(), Type: item.Type, ObtainedAt: at}
	switch rule.Type {
	case "required_status_checks":
		count, ok := countChecks(item.Parameters.Checks)
		if item.Parameters.Checks == nil || !ok {
			return domain.BranchRule{}, false
		}
		rule.RequiredChecks = count
	case "workflows":
		if item.Parameters.Workflows == nil {
			return domain.BranchRule{}, false
		}
		for _, workflow := range item.Parameters.Workflows {
			if !validID(workflow.RepositoryID.String()) || strings.TrimSpace(workflow.Path) == "" {
				return domain.BranchRule{}, false
			}
		}
		rule.RequiredChecks = len(item.Parameters.Workflows)
	}
	return rule, rule.Valid()
}

func (b *ciBudget) collectRules(ctx context.Context, root string, repository domain.RepositoryMetadata, ci *domain.GitHubCI) domain.SourceState {
	ctx = domain.WithHTTPInputBudget(ctx)
	if repository.DefaultBranch == nil {
		return ciSource(domain.BranchRulesScope, "GITHUB_CI_BRANCH_UNAVAILABLE", time.Time{}, 0, 0, 0)
	}
	branch := url.PathEscape(*repository.DefaultBranch)
	classic, at, code := b.classicChecks(ctx, root+"/branches/"+branch, *repository.DefaultBranch)
	ci.ClassicRequiredChecks = classic
	if classic != nil {
		classicAt := at
		ci.ClassicObtainedAt = &classicAt
	}
	path := root + "/rules/branches/" + branch
	type row struct {
		rule   domain.BranchRule
		params []byte
	}
	rules := map[ruleKey]row{}
	rejected := map[ruleKey]bool{}
	conflicts := 0
	invalid := 0
	for page := 1; ; page++ {
		response, failed := b.get(ctx, pagePath(path, nil, page))
		if failed != "" {
			if code == "" {
				code = failed
			}
			break
		}
		items, ok := decodeCIArray(response.body)
		if !ok || len(items) > ciPageSize {
			if code == "" {
				code = "GITHUB_CI_INVALID_RESPONSE"
			}
			break
		}
		at = response.obtainedAt
		overflow := false
		for _, raw := range items {
			k := readRuleKey(raw)
			if k.id != "" && !domain.RecordGitHubObject(ctx, path+"/"+k.id+"/"+k.kind) {
				overflow = true
				break
			}
			if rejected[k] {
				continue
			}
			rule, valid := decodeRule(raw, at)
			params, paramsValid := ruleParameters(raw)
			if !valid || !paramsValid {
				invalid++
				if k.id != "" {
					delete(rules, k)
					rejected[k] = true
				}
				continue
			}
			previous, exists := rules[k]
			if exists && !bytes.Equal(previous.params, params) {
				delete(rules, k)
				rejected[k] = true
				conflicts++
				continue
			}
			rules[k] = row{rule, params}
		}
		next, valid := nextCIPage(response.headers, path, repository.GitHubID, page)
		objectLimited := overflow || next && domain.GitHubObjectsExhausted(ctx)
		if objectLimited || page == domain.HTTPPageLimit {
			if code == "" {
				code = "GITHUB_CI_PAGE_LIMIT"
				if objectLimited {
					code = "GITHUB_CI_OBJECT_LIMIT"
				}
			}
			break
		}
		if !valid && code == "" {
			code = "GITHUB_CI_INVALID_PAGINATION"
		}
		if !next || !valid {
			break
		}
	}
	for _, rule := range rules {
		ci.Rules = append(ci.Rules, rule.rule)
	}
	sort.Slice(ci.Rules, func(i, j int) bool {
		a, b := ci.Rules[i], ci.Rules[j]
		return a.RulesetID < b.RulesetID || a.RulesetID == b.RulesetID && a.Type < b.Type
	})
	count := len(ci.Rules)
	if classic != nil {
		count++
	}
	return ciSource(domain.BranchRulesScope, code, at, count, invalid, conflicts)
}

func (b *ciBudget) classicChecks(ctx context.Context, path, branch string) (*int, time.Time, string) {
	response, code := b.get(ctx, path)
	if code != "" {
		return nil, time.Time{}, code
	}
	var info struct {
		Name      string `json:"name"`
		Protected *bool  `json:"protected"`
	}
	if json.Unmarshal(response.body, &info) != nil || info.Name != branch || info.Protected == nil {
		return nil, time.Time{}, "GITHUB_CI_INVALID_RESPONSE"
	}
	if !domain.RecordGitHubObject(ctx, path) {
		return nil, time.Time{}, "GITHUB_CI_OBJECT_LIMIT"
	}
	if !*info.Protected {
		zero := 0
		return &zero, response.obtainedAt, ""
	}
	observed := response.obtainedAt
	response, code = b.get(ctx, path+"/protection/required_status_checks")
	if code != "" {
		var failure struct {
			Message string `json:"message"`
		}
		// A generic 404 can hide missing Administration permission.
		if response.status == http.StatusNotFound && json.Unmarshal(response.body, &failure) == nil &&
			(failure.Message == "Required status checks not enabled" || failure.Message == "Branch not protected") {
			zero := 0
			return &zero, response.obtainedAt, ""
		}
		return nil, observed, code
	}
	var protection struct {
		Contexts []string        `json:"contexts"`
		Checks   []requiredCheck `json:"checks"`
	}
	if json.Unmarshal(response.body, &protection) != nil || protection.Contexts == nil && protection.Checks == nil {
		return nil, observed, "GITHUB_CI_INVALID_RESPONSE"
	}
	contexts := map[string]bool{}
	for _, name := range protection.Contexts {
		if strings.TrimSpace(name) == "" {
			return nil, observed, "GITHUB_CI_INVALID_RESPONSE"
		}
		contexts[name] = true
	}
	for _, check := range protection.Checks {
		if strings.TrimSpace(check.Context) == "" {
			return nil, observed, "GITHUB_CI_INVALID_RESPONSE"
		}
		contexts[check.Context] = true
	}
	count := len(contexts)
	if !domain.RecordGitHubObject(ctx, path+"/protection/required_status_checks") {
		return nil, observed, "GITHUB_CI_OBJECT_LIMIT"
	}
	return &count, response.obtainedAt, ""
}
