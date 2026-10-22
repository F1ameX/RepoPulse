package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/github"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type contextResolverFunc func(context.Context, SessionTokens) (domain.VerifiedContext, error)

func (f contextResolverFunc) ResolveContext(ctx context.Context, tokens SessionTokens) (domain.VerifiedContext, error) {
	return f(ctx, tokens)
}

func activeGuestContext() domain.VerifiedContext {
	guestID := "cfe8ce89-aec5-4773-a951-6d1b45420258"
	expiresAt := time.Now().Add(time.Hour)
	return domain.VerifiedContext{GuestSessionID: &guestID, GuestExpiresAt: &expiresAt}
}

func TestRepositoryValidationHTTPToGitHub(t *testing.T) {
	for _, tc := range []struct {
		name, body       string
		upstream, status int
		code             string
		calls            int
	}{
		{"public", `{"repository_url":" https://GitHub.com/Owner/Repo.git/ "}`, 200, 200, "", 1},
		{"empty", `{"repository_url":""}`, 0, 400, "URL_REQUIRED", 0},
		{"other host", `{"repository_url":"https://gitlab.com/a/b"}`, 0, 400, "UNSUPPORTED_PLATFORM", 0},
		{"file link", `{"repository_url":"https://github.com/a/b/blob/main/go.mod"}`, 0, 400, "INVALID_REPOSITORY_URL", 0},
		{"not found", `{"repository_url":"https://github.com/owner/repo"}`, 404, 404, "REPOSITORY_NOT_FOUND_OR_PRIVATE", 1},
		{"temporary failure", `{"repository_url":"https://github.com/owner/repo"}`, 503, 503, "REPOSITORY_CHECK_FAILED", 1},
		{"rate limit", `{"repository_url":"https://github.com/owner/repo"}`, 429, 503, "UPSTREAM_RATE_LIMITED", 1},
		{"timeout", `{"repository_url":"https://github.com/owner/repo"}`, -1, 504, "UPSTREAM_TIMEOUT", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := github.NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != "https://api.github.com/repos/owner/repo" {
					t.Fatalf("unexpected request %s", r.URL)
				}
				if tc.upstream == -1 {
					return nil, context.DeadlineExceeded
				}
				payload := `{"message":"upstream-secret-must-not-leak"}`
				if tc.upstream == 200 {
					payload = `{"id":42,"name":"Repo","full_name":"Owner/Repo","owner":{"login":"Owner"},"html_url":"https://github.com/Owner/Repo","private":false,"visibility":"public","default_branch":"main"}`
				}
				return &http.Response{StatusCode: tc.upstream, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
			})}, "service-token-must-not-leak")
			resolver := contextResolverFunc(func(_ context.Context, tokens SessionTokens) (domain.VerifiedContext, error) {
				if tokens.GuestToken == nil || *tokens.GuestToken != "opaque-guest-token" || tokens.AccountToken != nil {
					t.Fatalf("unexpected session tokens: %+v", tokens)
				}
				return activeGuestContext(), nil
			})
			handler := NewHandler(repository.NewValidator(client, 100*time.Millisecond), resolver)
			ctx := logger.With(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
			request := httptest.NewRequestWithContext(ctx, "POST", "/api/v1/repositories/validate", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: "opaque-guest-token"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status || calls != tc.calls {
				t.Fatalf("status=%d, calls=%d, body=%s", recorder.Code, calls, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "must-not-leak") {
				t.Fatal("source error or credentials leaked")
			}
			if tc.status == 200 {
				var result validatedRepositoryDTO
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Repository.GitHubID != "42" || result.Repository.CanonicalURL != "https://github.com/owner/repo" || result.Repository.Platform != "github" {
					t.Fatalf("bad DTO: %+v", result)
				}
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
					t.Fatal(err)
				}
				if string(raw["moved_from"]) != "null" || !strings.Contains(string(raw["repository"]), `"commit_sha":null`) {
					t.Fatal("nullable fields omitted")
				}
				return
			}
			var result errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Error.Code != tc.code || result.Error.TraceID == "" || result.Error.TraceID != recorder.Header().Get("X-Request-ID") || result.Error.FieldErrors == nil {
				t.Fatalf("bad error envelope: %+v", result)
			}
			if tc.status == 400 && (len(result.Error.FieldErrors) != 1 || result.Error.FieldErrors[0].Field != "repository_url") {
				t.Fatal("missing URL field error")
			}
		})
	}
}

func TestValidationRejectsMalformedJSONBeforeCallingApplication(t *testing.T) {
	for _, body := range []string{
		"", `{}`, `null`, `[]`, `{"repository_url":null}`, `{"repository_url":1}`, `{"repository_url":false}`,
		`{"Repository_Url":"https://github.com/a/b"}`, `{"repository_url":"a","repository_url":"b"}`,
		`{"repository_url":"a","owner":"spoofed"}`, `{"repository_url":"a","user_id":"spoofed"}`,
		`{"repository_url":"a","context":{}}`, `{"repository_url":"a"} {}`, `{"repository_url":`,
	} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/api/v1/repositories/validate", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json; charset=utf-8")
			recorder := httptest.NewRecorder()
			NewHandler(nil, nil).ServeHTTP(recorder, request)
			if recorder.Code != 400 || !strings.Contains(recorder.Body.String(), `"code":"VALIDATION_FAILED"`) {
				t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	for _, tc := range []struct {
		contentType, body string
		status            int
	}{
		{"text/plain", `{"repository_url":"a"}`, 400},
		{"", `{"repository_url":"a"}`, 400},
		{"application/json", strings.Repeat(" ", (1<<20)+1), 400},
	} {
		request := httptest.NewRequest("POST", "/api/v1/repositories/validate", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", tc.contentType)
		recorder := httptest.NewRecorder()
		NewHandler(nil, nil).ServeHTTP(recorder, request)
		if recorder.Code != tc.status {
			t.Fatalf("status=%d, want %d", recorder.Code, tc.status)
		}
	}
}

func TestValidationDoesNotBypassAuth(t *testing.T) {
	for _, tc := range []struct {
		name, cookies, code string
		resolver            ContextResolver
		status              int
	}{
		{"no session", "", "SESSION_REQUIRED", contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) {
			t.Fatal("Auth called without session tokens")
			return domain.VerifiedContext{}, nil
		}), 401},
		{"unrelated cookie", "other=value", "SESSION_REQUIRED", nil, 401},
		{"Auth not connected", "repopulse_guest=opaque", "NOT_IMPLEMENTED", nil, 501},
		{"expired account with guest", "repopulse_session=expired; repopulse_guest=opaque", "SESSION_EXPIRED", contextResolverFunc(func(_ context.Context, tokens SessionTokens) (domain.VerifiedContext, error) {
			if tokens.AccountToken == nil || *tokens.AccountToken != "expired" || tokens.GuestToken == nil || *tokens.GuestToken != "opaque" {
				t.Fatal("both cookies must be passed to Auth without fallback")
			}
			return domain.VerifiedContext{}, domain.ErrSessionExpired
		}), 401},
		{"empty Auth context", "repopulse_guest=opaque", "SESSION_REQUIRED", contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) {
			return domain.VerifiedContext{}, nil
		}), 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("GitHub called without a verified session")
				return nil, context.Canceled
			})}, "")
			request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories/validate", strings.NewReader(`{"repository_url":"https://github.com/owner/repo"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Cookie", tc.cookies)
			recorder := httptest.NewRecorder()
			NewHandler(repository.NewValidator(client, time.Second), tc.resolver).ServeHTTP(recorder, request)
			var result errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || result.Error.Code != tc.code || result.Error.FieldErrors == nil || result.Error.TraceID == "" {
				t.Fatalf("status=%d, error=%+v", recorder.Code, result.Error)
			}
		})
	}
}
