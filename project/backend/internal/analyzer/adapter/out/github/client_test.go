package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: headers}
}

const publicMetadata = `{"id":9007199254740993,"name":"Repo","full_name":"Owner/Repo","owner":{"login":"Owner"},"html_url":"https://github.com/Owner/Repo","private":false,"visibility":"public","default_branch":"main"}`

func TestMetadataConfirmsPublicIncludingEmptyForkArchived(t *testing.T) {
	for _, extra := range []string{"", `,"size":0,"default_branch":null`, `,"fork":true`, `,"archived":true`} {
		t.Run(extra, func(t *testing.T) {
			body := publicMetadata
			if strings.Contains(extra, "default_branch") {
				body = strings.Replace(body, `,"default_branch":"main"`, "", 1)
			}
			body = strings.TrimSuffix(body, "}") + extra + "}"
			client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != apiOrigin+"/repos/owner/repo" || r.Method != "GET" ||
					r.Header.Get("Authorization") != "Bearer service-token" || r.Header.Get("Accept") != "application/vnd.github+json" ||
					r.Header.Get("User-Agent") == "" || r.Header.Get("X-GitHub-Api-Version") == "" {
					t.Fatalf("unexpected metadata request: %s", r.URL)
				}
				return response(200, body, nil), nil
			})}, "service-token")
			got, err := client.Lookup(context.Background(), domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if err != nil || got.Repository.GitHubID != "9007199254740993" || got.Repository.CanonicalURL != "https://github.com/owner/repo" ||
				got.Repository.Owner != "Owner" || got.Repository.Name != "Repo" || got.Repository.CommitSHA != nil || got.MovedFrom != nil {
				t.Fatalf("result=%+v, error=%v", got, err)
			}
			if extra == "" && (got.Repository.DefaultBranch == nil || *got.Repository.DefaultBranch != "main") {
				t.Fatal("lost default branch")
			}
			if strings.Contains(extra, "default_branch") && got.Repository.DefaultBranch != nil {
				t.Fatal("empty repo branch must be null")
			}
		})
	}
}

func TestGitHubFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		headers http.Header
		want    error
	}{
		{"missing", 404, `{"message":"Not Found"}`, nil, domain.ErrRepositoryUnavailable},
		{"private", 200, strings.Replace(publicMetadata, `"private":false`, `"private":true`, 1), nil, domain.ErrRepositoryUnavailable},
		{"internal visibility", 200, strings.Replace(publicMetadata, `"public"`, `"internal"`, 1), nil, domain.ErrRepositoryUnavailable},
		{"forbidden", 403, `{"message":"Forbidden"}`, nil, domain.ErrRepositoryUnavailable},
		{"blocked", 403, `{"message":"Repository access blocked"}`, nil, domain.ErrRepositoryUnavailable},
		{"service token", 401, `{"message":"Bad credentials secret-token"}`, nil, domain.ErrRepositoryCheck},
		{"service permission", 403, `{"message":"Resource not accessible by integration"}`, nil, domain.ErrRepositoryCheck},
		{"unconfirmed denial", 403, `<html>proxy denied</html>`, nil, domain.ErrRepositoryCheck},
		{"429", 429, `{}`, nil, domain.ErrUpstreamRateLimited},
		{"primary limit", 403, `{}`, http.Header{"X-Ratelimit-Remaining": {"0"}}, domain.ErrUpstreamRateLimited},
		{"secondary limit", 403, `{"message":"You have exceeded a secondary rate limit"}`, nil, domain.ErrUpstreamRateLimited},
		{"retry after", 403, `{}`, http.Header{"Retry-After": {"60"}}, domain.ErrUpstreamRateLimited},
		{"5xx", 503, `<html>unavailable</html>`, nil, domain.ErrRepositoryCheck},
		{"HTML success", 200, `<html>repository page</html>`, nil, domain.ErrRepositoryCheck},
		{"no public flag", 200, strings.Replace(publicMetadata, `"private":false,`, "", 1), nil, domain.ErrRepositoryCheck},
		{"empty metadata", 200, `{}`, nil, domain.ErrRepositoryCheck},
		{"inconsistent name", 200, strings.Replace(publicMetadata, `"full_name":"Owner/Repo"`, `"full_name":"Other/Repo"`, 1), nil, domain.ErrRepositoryCheck},
		{"external URL", 200, strings.Replace(publicMetadata, "https://github.com/", "https://example.com/", 1), nil, domain.ErrRepositoryCheck},
		{"unconfirmed move", 200, strings.ReplaceAll(publicMetadata, "Owner", "Other"), nil, domain.ErrRepositoryCheck},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return response(tc.status, tc.body, tc.headers), nil
			})}, "")
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, err := client.Lookup(ctx, domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("error=%v, calls=%d; want %v, 1", err, calls, tc.want)
			}
		})
	}
}

func TestOnlyConfirmedNumericIDRedirectIsFollowed(t *testing.T) {
	for _, tc := range []struct {
		name, location, body string
		want                 error
		calls                int
	}{
		{"confirmed move", apiOrigin + "/repositories/9007199254740993", strings.ReplaceAll(publicMetadata, "Owner", "NewOwner"), nil, 2},
		{"relative numeric", "/repositories/9007199254740993", strings.ReplaceAll(publicMetadata, "Owner", "NewOwner"), nil, 2},
		{"changed ID", apiOrigin + "/repositories/42", publicMetadata, domain.ErrRepositoryCheck, 2},
		{"private target", apiOrigin + "/repositories/9007199254740993", strings.Replace(publicMetadata, `"private":false`, `"private":true`, 1), domain.ErrRepositoryUnavailable, 2},
		{"different host", "https://evil.example/repositories/1", "", domain.ErrRepositoryCheck, 1},
		{"different scheme", "http://api.github.com/repositories/1", "", domain.ErrRepositoryCheck, 1},
		{"port", "https://api.github.com:443/repositories/1", "", domain.ErrRepositoryCheck, 1},
		{"userinfo", "https://u:p@api.github.com/repositories/1", "", domain.ErrRepositoryCheck, 1},
		{"unconfirmed name", apiOrigin + "/repos/newowner/repo", "", domain.ErrRepositoryCheck, 1},
		{"encoded path", apiOrigin + "/repositories/%31", "", domain.ErrRepositoryCheck, 1},
		{"query", apiOrigin + "/repositories/1?", "", domain.ErrRepositoryCheck, 1},
		{"fragment", apiOrigin + "/repositories/1#", "", domain.ErrRepositoryCheck, 1},
		{"loop", apiOrigin + "/repositories/9007199254740993", "redirect again", domain.ErrRepositoryCheck, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "api.github.com" {
					t.Fatal("request escaped GitHub")
				}
				if calls == 1 || tc.body == "redirect again" {
					return response(301, "", http.Header{"Location": {tc.location}}), nil
				}
				return response(200, tc.body, nil), nil
			})}, "service-token")
			got, err := client.Lookup(context.Background(), domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if !errors.Is(err, tc.want) || calls != tc.calls {
				t.Fatalf("got=%+v, error=%v, calls=%d", got, err, calls)
			}
			if err == nil && (got.MovedFrom == nil || *got.MovedFrom != "https://github.com/owner/repo" || got.Repository.CanonicalURL != "https://github.com/newowner/repo") {
				t.Fatalf("move not preserved: %+v", got)
			}
		})
	}
}

func TestRetriesAreBoundedAndRespectRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		headers http.Header
		recover bool
		want    error
		calls   int
	}{
		{"5xx recovery", 503, nil, true, nil, 2},
		{"permanent 5xx", 503, nil, false, domain.ErrRepositoryCheck, 2},
		{"retry immediately permitted", 429, http.Header{"Retry-After": {"0"}}, true, nil, 2},
		{"long wait", 429, http.Header{"Retry-After": {"120"}}, true, domain.ErrUpstreamRateLimited, 1},
		{"5xx retry after", 503, http.Header{"Retry-After": {"120"}}, true, domain.ErrRepositoryCheck, 1},
		{"primary reset elapsed", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1"}}, true, nil, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if tc.recover && calls == 2 {
					return response(200, publicMetadata, nil), nil
				}
				return response(tc.status, `{}`, tc.headers), nil
			})}, "")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := client.Lookup(ctx, domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if !errors.Is(err, tc.want) || calls != tc.calls {
				t.Fatalf("error=%v, calls=%d", err, calls)
			}
		})
	}
}

func TestNetworkFailuresTimeoutCancellationAndResponseLimit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport transportFunc
		want      error
	}{
		{"network error", func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("private transport detail") }, domain.ErrRepositoryCheck},
		{"timeout", func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded }, domain.ErrUpstreamTimeout},
		{"oversized response", func(*http.Request) (*http.Response, error) {
			return response(200, strings.Repeat("x", maxResponseBytes+1), nil), nil
		}, domain.ErrRepositoryCheck},
		{"deadline", func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() }, domain.ErrUpstreamTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(&http.Client{Transport: tc.transport}, "")
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := client.Lookup(ctx, domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })}, "")
	if _, err := client.Lookup(ctx, domain.RepositoryURL{Owner: "owner", Name: "repo"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
