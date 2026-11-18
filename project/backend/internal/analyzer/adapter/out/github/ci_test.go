package github

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const ciRepoPath = "/repos/owner/repo"

func ciTestInputs() (domain.RepositoryMetadata, domain.Window) {
	branch := "release/main"
	return domain.RepositoryMetadata{Platform: "github", GitHubID: "42", Owner: "owner", Name: "repo",
			CanonicalURL: "https://github.com/owner/repo", DefaultBranch: &branch}, domain.Window{
			From:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			Until: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
		}
}

func runFixture(id string, attempt, day int, conclusion string) string {
	return fmt.Sprintf(`{"id":%s,"run_attempt":%d,"head_branch":"release/main","head_sha":"%s","created_at":"2026-11-01T10:00:00Z","run_started_at":"2026-11-%02dT11:00:00Z","updated_at":"2026-11-%02dT11:30:00Z","status":"completed","conclusion":"%s"}`,
		id, attempt, strings.Repeat("a", 40), day, day, conclusion)
}

func runsFixture(total int, items ...string) string {
	return fmt.Sprintf(`{"total_count":%d,"workflow_runs":[%s]}`, total, strings.Join(items, ","))
}

func nextPageFixture(path string, page int) http.Header {
	return http.Header{"Link": {fmt.Sprintf("<%s%s?page=%d>; rel=\"next\"", apiOrigin, path, page)}}
}

func TestCollectCIPaginatesAndKeepsAttemptsSeparate(t *testing.T) {
	const largeID = "9007199254740993"
	repo, window := ciTestInputs()
	run1 := runFixture(largeID, 1, 1, "failure")
	run2 := runFixture(largeID, 2, 2, "success")
	const rulesPath = ciRepoPath + "/rules/branches/release%2Fmain"
	const runsPath = ciRepoPath + "/actions/runs"
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.EscapedPath() {
		case ciRepoPath + "/branches/release%2Fmain":
			return response(200, `{"name":"release/main","protected":false}`, nil), nil
		case rulesPath:
			if r.URL.Query().Get("page") == "1" {
				return response(200, `[{"ruleset_id":3,"type":"pull_request"}]`, nextPageFixture(rulesPath, 2)), nil
			}
			return response(200, `[{"ruleset_id":8,"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"build"}]}}]`, nil), nil
		case runsPath:
			if r.URL.Query().Get("created") != "2026-07-01T00:00:00Z..2026-11-18T00:00:00Z" || r.URL.Query().Get("branch") != "release/main" {
				t.Fatalf("unexpected window query: %s", r.URL)
			}
			if r.URL.Query().Get("page") == "1" {
				return response(200, runsFixture(3, run2, runFixture("12", 1, 1, "success")), nextPageFixture("/repositories/42/actions/runs", 2)), nil
			}
			return response(200, runsFixture(3, run2, runFixture("13", 1, 1, "neutral")), nil), nil
		case runsPath + "/" + largeID + "/attempts/1":
			return response(200, run1, nil), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})}, "")
	ci, sources := client.CollectCI(t.Context(), repo, window)
	if len(ci.Attempts) != 4 || len(ci.Rules) != 2 || sources[0].Status != "complete" || sources[1].Status != "complete" {
		t.Fatalf("CI=%+v sources=%+v", ci, sources)
	}
	latest, found := ci.LatestCompleted(window)
	checks := ci.RequiredChecks(sources[1])
	if !found || latest.RunID != largeID || latest.Attempt != 2 || latest.Conclusion != "success" || len(ci.CompletedAttempts(window)) != 3 ||
		latest.ObtainedAt.IsZero() || checks == nil || !*checks {
		t.Fatalf("latest=%+v checks=%v", latest, checks)
	}
}

func TestCollectCIPreservesPartialActionsAndUnknownProtection(t *testing.T) {
	repo, window := ciTestInputs()
	const runsPath = ciRepoPath + "/actions/runs"
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.EscapedPath() {
		case ciRepoPath + "/branches/release%2Fmain":
			return response(200, `{"name":"release/main","protected":true}`, nil), nil
		case ciRepoPath + "/branches/release%2Fmain/protection/required_status_checks":
			return response(404, `{"message":"Not Found"}`, nil), nil
		case ciRepoPath + "/rules/branches/release%2Fmain":
			return response(200, `[{"ruleset_id":8,"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"build"}]}}]`, nil), nil
		case runsPath:
			switch r.URL.Query().Get("page") {
			case "1":
				return response(200, runsFixture(4, runFixture("1", 1, 1, "success"), runFixture("2", 1, 1, "success")), nextPageFixture(runsPath, 2)), nil
			case "2":
				return response(200, runsFixture(4, runFixture("2", 1, 1, "failure"), `{"id":3}`), nextPageFixture(runsPath, 3)), nil
			default:
				return response(403, `{"message":"Resource not accessible"}`, nil), nil
			}
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})}, "")
	ci, sources := client.CollectCI(t.Context(), repo, window)
	actions, rules := sources[0], sources[1]
	if len(ci.Attempts) != 1 || ci.Attempts[0].RunID != "1" || actions.Status != "partial" ||
		actions.Reason.Code != "GITHUB_CI_ACCESS_UNAVAILABLE" || actions.Sample.Total != nil ||
		actions.Sample.Excluded == nil || *actions.Sample.Excluded != 2 || actions.ObtainedAt == nil {
		t.Fatalf("actions=%+v attempts=%+v", actions, ci.Attempts)
	}
	if len(ci.Rules) != 1 || rules.Status != "partial" || rules.Reason.Code != "GITHUB_CI_ACCESS_UNAVAILABLE" || ci.RequiredChecks(rules) != nil {
		t.Fatalf("unknown protection became false: %+v", rules)
	}
}

func TestCollectCIDistinguishesEmptyRunsFromDeniedAccess(t *testing.T) {
	for _, status := range []int{200, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repo, window := ciTestInputs()
			client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.EscapedPath() {
				case ciRepoPath + "/branches/release%2Fmain":
					return response(200, `{"name":"release/main","protected":false}`, nil), nil
				case ciRepoPath + "/rules/branches/release%2Fmain":
					return response(200, `[]`, nil), nil
				default:
					return response(status, runsFixture(0), nil), nil
				}
			})}, "")
			ci, sources := client.CollectCI(t.Context(), repo, window)
			present := ci.RunsPresent(sources[0])
			if status == 200 && (present == nil || *present || sources[0].Status != "complete") ||
				status == 403 && (present != nil || sources[0].Status != "unavailable") {
				t.Fatalf("status %d: actions=%+v present=%v", status, sources[0], present)
			}
		})
	}
}

func TestActionsRetainsFirstPageWhenSecondResponseIsTruncated(t *testing.T) {
	repo, window := ciTestInputs()
	const path = ciRepoPath + "/actions/runs"
	calls := 0
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.Query().Get("page") {
		case "1":
			return response(200, runsFixture(2, runFixture("1", 1, 1, "success")), nextPageFixture(path, 2)), nil
		case "2":
			reply := response(200, "", nil)
			reply.Body = io.NopCloser(io.MultiReader(strings.NewReader(`{"total_count":2,"workflow_runs":[`), iotest.ErrReader(io.ErrUnexpectedEOF)))
			return reply, nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})}, "")
	budget := ciBudget{client: client, remaining: ciAttempts}
	var ci domain.GitHubCI
	got := budget.collectActions(t.Context(), ciRepoPath, repo, window, &ci)
	if calls != 3 || len(ci.Attempts) != 1 || ci.Attempts[0].RunID != "1" ||
		got.Status != "partial" || got.Reason == nil || got.Reason.Code != "GITHUB_CI_INTERRUPTED" || got.Sample.Total != nil {
		t.Fatalf("calls=%d attempts=%+v source=%+v", calls, ci.Attempts, got)
	}
}

func TestCIAttemptsCountRetriesWithoutRequestAfterBudget(t *testing.T) {
	calls := 0
	client := NewClient(&http.Client{Transport: transportFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return response(429, "", http.Header{"Retry-After": {"0"}}), nil
	})}, "")
	budget := ciBudget{client: client, remaining: 2}
	_, code := budget.get(t.Context(), ciRepoPath+"/actions/runs")
	if code != "GITHUB_CI_RATE_LIMITED" || calls != 2 || budget.remaining != 0 {
		t.Fatalf("code=%s calls=%d remaining=%d", code, calls, budget.remaining)
	}
	budget.get(t.Context(), ciRepoPath+"/actions/runs?page=2")
	if calls != 2 {
		t.Fatal("a request was sent after the budget")
	}
}

func TestActionsStopsAtPageLimitAndExcludesRightBound(t *testing.T) {
	repo, window := ciTestInputs()
	const path = ciRepoPath + "/actions/runs"
	calls := 0
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > domain.HTTPPageLimit || r.URL.Path != path {
			t.Fatalf("unexpected request %s", r.URL)
		}
		item := runFixture(strconv.Itoa(calls), 1, calls, "success")
		if calls == 1 {
			item = strings.Replace(item, "2026-11-01T10:00:00Z", window.Until.Format(time.RFC3339), 1)
			item = strings.Replace(item, "2026-11-01T11:00:00Z", window.Until.Format(time.RFC3339), 1)
			item = strings.Replace(item, "2026-11-01T11:30:00Z", window.Until.Format(time.RFC3339), 1)
		}
		return response(200, runsFixture(11, item), nextPageFixture(path, calls+1)), nil
	})}, "")
	budget := ciBudget{client: client, remaining: ciAttempts}
	var ci domain.GitHubCI
	got := budget.collectActions(t.Context(), ciRepoPath, repo, window, &ci)
	if calls != domain.HTTPPageLimit || len(ci.Attempts) != 9 || got.Status != "partial" ||
		got.Reason == nil || got.Reason.Code != "GITHUB_CI_PAGE_LIMIT" || got.Sample.Total != nil {
		t.Fatalf("calls=%d attempts=%d source=%+v", calls, len(ci.Attempts), got)
	}
}

func TestActionsCountsUniqueAttemptsAgainstRemainingGitHubObjects(t *testing.T) {
	repo, window := ciTestInputs()
	item := runFixture("1", 1, 1, "success")
	const path = ciRepoPath + "/actions/runs"
	calls := 0
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls != 1 || r.URL.Path != path {
			t.Fatalf("unexpected request %s", r.URL)
		}
		return response(200, runsFixture(4, item, item, runFixture("2", 1, 1, "success"), runFixture("3", 1, 1, "success")), nextPageFixture(path, 2)), nil
	})}, "")
	budget := ciBudget{client: client, remaining: ciAttempts, objects: ciObjects - 2}
	var ci domain.GitHubCI
	got := budget.collectActions(t.Context(), ciRepoPath, repo, window, &ci)
	if calls != 1 || len(ci.Attempts) != 2 || got.Status != "partial" ||
		got.Reason == nil || got.Reason.Code != "GITHUB_CI_OBJECT_LIMIT" {
		t.Fatalf("calls=%d attempts=%d source=%+v", calls, len(ci.Attempts), got)
	}
}
