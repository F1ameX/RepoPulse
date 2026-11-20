package github

import (
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
	type key struct{ id, kind string }
	rules := map[key]domain.BranchRule{}
	conflicts := map[key]bool{}
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
		for _, raw := range items {
			rule, valid := decodeRule(raw, at)
			if !valid {
				invalid++
				continue
			}
			k := key{rule.RulesetID, rule.Type}
			if conflicts[k] {
				continue
			}
			previous, exists := rules[k]
			if exists && previous.RequiredChecks != rule.RequiredChecks {
				delete(rules, k)
				conflicts[k] = true
				continue
			}
			if !exists {
				if b.objects == ciObjects {
					break
				}
				b.objects++
			}
			rules[k] = rule
		}
		next, valid := nextCIPage(response.headers, path, repository.GitHubID, page)
		if b.objects == ciObjects || page == domain.HTTPPageLimit {
			if code == "" {
				code = "GITHUB_CI_PAGE_LIMIT"
				if b.objects == ciObjects {
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
		ci.Rules = append(ci.Rules, rule)
	}
	sort.Slice(ci.Rules, func(i, j int) bool {
		a, b := ci.Rules[i], ci.Rules[j]
		return a.RulesetID < b.RulesetID || a.RulesetID == b.RulesetID && a.Type < b.Type
	})
	count := len(ci.Rules)
	if classic != nil {
		count++
	}
	return ciSource(domain.BranchRulesScope, code, at, count, invalid, len(conflicts))
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
	b.objects++ // The branch metadata is a separate GitHub object.
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
	b.objects++
	return &count, response.obtainedAt, ""
}
