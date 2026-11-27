package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const (
	ciAttempts = domain.GitHubHTTPLimit
	ciPageSize = 100
)

type ciBudget struct {
	client    *Client
	remaining int
	blocked   string
}

// CollectCI shares its budget across branch rules, run pages and older attempts.
// A source failure is returned as a source state so other data remains usable.
func (c *Client) CollectCI(ctx context.Context, repository domain.RepositoryMetadata, window domain.Window) (domain.GitHubCI, []domain.SourceState) {
	ctx = domain.WithHTTPInputBudget(ctx)
	timeout := domain.CoreTimeLimit
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, max(0, time.Until(deadline)-5*time.Second)/2)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	root := "/repos/" + url.PathEscape(repository.Owner) + "/" + url.PathEscape(repository.Name)
	ci := domain.GitHubCI{Attempts: []domain.ActionsAttempt{}, Rules: []domain.BranchRule{}}
	if repository.Validate() != nil || !validID(repository.GitHubID) || !window.From.Before(window.Until) {
		return ci, []domain.SourceState{
			ciSource(domain.ActionsScope, "GITHUB_CI_INPUT_INVALID", time.Time{}, 0, 0, 0),
			ciSource(domain.BranchRulesScope, "GITHUB_CI_INPUT_INVALID", time.Time{}, 0, 0, 0),
		}
	}
	// Reserve time and requests for Actions even if branch rules are lengthy.
	remaining := domain.HTTPAttemptsRemaining(ctx, domain.HTTPGitHub)
	rulesAttempts := max(0, remaining-1)
	rulesCtx, cancelRules := context.WithTimeout(ctx, timeout/2)
	rulesBudget := ciBudget{client: c, remaining: rulesAttempts}
	rules := rulesBudget.collectRules(rulesCtx, root, repository, &ci)
	cancelRules()
	budget := ciBudget{client: c, remaining: remaining - rulesAttempts + rulesBudget.remaining, blocked: rulesBudget.blocked}
	actions := budget.collectActions(ctx, root, repository, window, &ci)
	return ci, []domain.SourceState{actions, rules}
}

type ciResponse struct {
	body       []byte
	headers    http.Header
	status     int
	obtainedAt time.Time
}

func (b *ciBudget) get(ctx context.Context, path string) (ciResponse, string) {
	for attempt := 1; ; attempt++ {
		switch {
		case ctx.Err() != nil:
			return ciResponse{}, "GITHUB_CI_TIMEOUT"
		case b.blocked != "":
			return ciResponse{}, b.blocked
		case b.remaining == 0:
			return ciResponse{}, "GITHUB_CI_REQUEST_LIMIT"
		case domain.GitHubObjectsExhausted(ctx):
			return ciResponse{}, "GITHUB_CI_OBJECT_LIMIT"
		case domain.HTTPInputExhausted(ctx):
			return ciResponse{}, "GITHUB_CI_INPUT_LIMIT"
		}
		b.remaining--
		body, status, headers, err := b.client.attempt(ctx, path)
		response := ciResponse{body, headers, status, domain.UTC(time.Now())}
		delay := time.Second
		code := ""
		switch {
		case errors.Is(err, domain.ErrHTTPAttemptLimit):
			return response, "GITHUB_CI_REQUEST_LIMIT"
		case errors.Is(err, domain.ErrHTTPInputLimit):
			return response, "GITHUB_CI_INPUT_LIMIT"
		case errors.Is(err, domain.ErrHTTPResponseLimit):
			return response, "GITHUB_CI_RESPONSE_LIMIT"
		case status == http.StatusTooManyRequests || status == http.StatusForbidden && rateLimited(headers, body):
			code, delay = "GITHUB_CI_RATE_LIMITED", rateLimitDelay(headers)
		case status >= 500:
			code = "GITHUB_CI_UPSTREAM_FAILED"
			if headers.Get("Retry-After") != "" {
				delay = max(delay, rateLimitDelay(headers))
				code = "GITHUB_CI_RETRY_DEFERRED"
			}
		case errors.Is(err, domain.ErrUpstreamTimeout), errors.Is(err, context.DeadlineExceeded):
			code = "GITHUB_CI_TIMEOUT"
		case err != nil:
			code = "GITHUB_CI_INTERRUPTED"
		case status == http.StatusOK:
			return response, ""
		case status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound:
			return response, "GITHUB_CI_ACCESS_UNAVAILABLE"
		default:
			return response, "GITHUB_CI_HTTP_ERROR"
		}
		available := domain.HTTPAttemptAvailable(ctx, domain.HTTPGitHub)
		if attempt == domain.HTTPMaxTries || b.remaining == 0 || available != nil || !domain.HTTPRetryFits(ctx, delay) {
			if code == "GITHUB_CI_RATE_LIMITED" || code == "GITHUB_CI_RETRY_DEFERRED" {
				b.blocked = code
			}
			if attempt < domain.HTTPMaxTries && b.blocked == "" && (b.remaining == 0 || errors.Is(available, domain.ErrHTTPAttemptLimit)) {
				return response, "GITHUB_CI_REQUEST_LIMIT"
			}
			if errors.Is(available, domain.ErrHTTPInputLimit) {
				return response, "GITHUB_CI_INPUT_LIMIT"
			}
			return response, code
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return response, "GITHUB_CI_TIMEOUT"
		case <-timer.C:
		}
	}
}

func pagePath(path string, filters url.Values, page int) string {
	query := url.Values{"per_page": {strconv.Itoa(ciPageSize)}, "page": {strconv.Itoa(page)}}
	for k, values := range filters {
		query[k] = values
	}
	return path + "?" + query.Encode()
}

// Read only the page number from Link. Rebuild the request from the original
// endpoint and filters, so upstream cannot change its destination or scope.
func nextCIPage(headers http.Header, path, repoID string, page int) (bool, bool) {
	_, suffix, _ := strings.Cut(strings.TrimPrefix(path, "/repos/"), "/")
	_, suffix, _ = strings.Cut(suffix, "/")
	alternate := "/repositories/" + repoID + "/" + suffix
	found := false
	for _, value := range headers.Values("Link") {
		for _, link := range strings.Split(value, ",") {
			link = strings.TrimSpace(link)
			target, attributes, ok := strings.Cut(strings.TrimPrefix(link, "<"), ">")
			if !ok || !strings.HasPrefix(link, "<") {
				return false, false
			}
			for _, attribute := range strings.Split(attributes, ";") {
				name, value, _ := strings.Cut(strings.TrimSpace(attribute), "=")
				if name != "rel" || strings.Trim(value, "\"") != "next" {
					continue
				}
				u, err := url.Parse(target)
				if found || err != nil || u.Scheme != "https" || u.Host != "api.github.com" || u.User != nil ||
					u.Fragment != "" || (u.EscapedPath() != path && u.EscapedPath() != alternate) ||
					u.Query().Get("page") != strconv.Itoa(page+1) {
					return false, false
				}
				found = true
			}
		}
	}
	return found, true
}

func ciSource(scope, code string, at time.Time, count, invalid, conflicts int) domain.SourceState {
	state := domain.SourceState{Source: "github", Scope: scope, Status: "complete"}
	if !at.IsZero() {
		state.ObtainedAt = &at
		state.Sample.Collected = &count
	}
	if invalid+conflicts > 0 {
		excluded := invalid + conflicts
		exclusion := "GITHUB_CI_INVALID_RECORD"
		if conflicts > 0 {
			exclusion = "GITHUB_CI_CONFLICTING_RECORD"
			if invalid > 0 {
				exclusion = "GITHUB_CI_INVALID_AND_CONFLICTING_RECORDS"
			}
		}
		state.Sample.Excluded, state.Sample.ExcludedReason = &excluded, &exclusion
		if code == "" {
			code = exclusion
		}
	}
	if code == "" {
		state.Sample.Total = &count
		return state
	}
	state.Status = "unavailable"
	if state.ObtainedAt != nil {
		state.Status = "partial"
	}
	state.Reason = &domain.Reason{Code: code, Message: "Данные GitHub CI/CD получены не полностью",
		CannotConclude: "Нельзя делать вывод об отсутствии запусков или обязательных проверок по ограниченному источнику"}
	return state
}

func decodeCIArray(body []byte) ([]json.RawMessage, bool) {
	var items []json.RawMessage
	err := json.Unmarshal(body, &items)
	return items, err == nil && items != nil
}
