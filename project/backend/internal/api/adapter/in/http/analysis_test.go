package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

type analysisScenarios struct {
	create func(context.Context, analysis.CreateAnalysisCommand) (domain.Analysis, error)
	get    func(context.Context, analysis.GetAnalysisQuery) (domain.Analysis, error)
}

func (s analysisScenarios) CreateAnalysis(ctx context.Context, command analysis.CreateAnalysisCommand) (domain.Analysis, error) {
	return s.create(ctx, command)
}

func (s analysisScenarios) GetAnalysis(ctx context.Context, query analysis.GetAnalysisQuery) (domain.Analysis, error) {
	return s.get(ctx, query)
}

func TestAnalysisHTTPPreservesNullableFieldsAndFailedIsReadable(t *testing.T) {
	for _, state := range []string{"queued", "running", "completed", "failed"} {
		t.Run(state, func(t *testing.T) {
			id := domain.NewID()
			a := domain.Analysis{ID: id, Status: state, CreatedAt: time.Now().UTC(),
				Repository: domain.RepositoryMetadata{Platform: "github", GitHubID: "42", CanonicalURL: "https://github.com/owner/repo", Owner: "Owner", Name: "Repo"}}
			if state == "failed" {
				a.Failure = &domain.Failure{Code: "BUDGET_EXCEEDED", TraceID: "original-diagnostic-id"}
			}
			if state == "completed" {
				reportID, at := domain.NewID(), time.Now().UTC()
				a.ReportID, a.CompletedAt = &reportID, &at
			}
			calls := 0
			scenarios := analysisScenarios{get: func(_ context.Context, query analysis.GetAnalysisQuery) (domain.Analysis, error) {
				calls++
				if query.AnalysisID != id || query.Context.GuestSessionID == nil {
					t.Fatal("query lost verified context or ID")
				}
				return a, nil
			}}
			resolver := contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) { return activeGuestContext(), nil })
			handler := NewAnalysisHandler(nil, resolver, scenarios, "http://localhost:5173")
			for range 2 {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/analyses/"+id, nil)
				request.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: "opaque"})
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
					t.Fatal(err)
				}
				if recorder.Code != http.StatusOK || len(raw) != 9 || string(raw["stage"]) != "null" || string(raw["started_at"]) != "null" {
					t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
				}
				if state == "failed" && (!strings.Contains(string(raw["error"]), `"code":"BUDGET_EXCEEDED"`) || !strings.Contains(string(raw["error"]), `"field_errors":[]`)) {
					t.Fatalf("terminal error lost: %s", recorder.Body.String())
				}
				if state != "failed" && string(raw["error"]) != "null" {
					t.Fatal("computation failure invented")
				}
			}
			if calls != 2 {
				t.Fatalf("status reads=%d", calls)
			}
		})
	}
}

func TestAnalysisReadFailureAndAuthUnavailableAreTransportErrors(t *testing.T) {
	for _, tc := range []struct {
		name             string
		authErr, readErr error
		code             string
		status           int
	}{
		{"auth unavailable", apidomain.ErrAuthUnavailable, nil, "AUTH_UNAVAILABLE", 503},
		{"session expired", domain.ErrSessionExpired, nil, "SESSION_EXPIRED", 401},
		{"Auth session expired", apidomain.ErrSessionExpired, nil, "SESSION_EXPIRED", 401},
		{"database read", nil, errors.New("private database detail"), "INTERNAL_ERROR", 500},
		{"foreign or missing", nil, domain.ErrNotFound, "ANALYSIS_NOT_FOUND", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := analysisScenarios{get: func(context.Context, analysis.GetAnalysisQuery) (domain.Analysis, error) {
				calls++
				return domain.Analysis{}, tc.readErr
			}}
			resolver := contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) {
				return activeGuestContext(), tc.authErr
			})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/analyses/"+domain.NewID(), nil)
			request.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: "token-must-not-leak"})
			recorder := httptest.NewRecorder()
			NewAnalysisHandler(nil, resolver, service, "http://localhost:5173").ServeHTTP(recorder, request)
			if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), `"code":"`+tc.code+`"`) ||
				strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "must-not-leak") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if tc.authErr != nil && calls != 0 {
				t.Fatal("analysis read before successful Auth verification")
			}
		})
	}
}

func TestCreateAnalysisRejectsUntrustedOriginAndMalformedBody(t *testing.T) {
	requestID := domain.NewID()
	valid := `{"request_id":"` + requestID + `","repository_url":"https://github.com/owner/repo"}`
	for _, tc := range []struct {
		body, origin string
		status       int
	}{
		{valid, "https://other.example", 403}, {valid, "", 403},
		{`{"request_id":null,"repository_url":"x"}`, "http://localhost:5173", 400},
		{`{"Request_Id":"x","repository_url":"x"}`, "http://localhost:5173", 400},
		{`{"request_id":"x","request_id":"y","repository_url":"x"}`, "http://localhost:5173", 400},
		{`{"request_id":"x","repository_url":"x","owner":"spoofed"}`, "http://localhost:5173", 400},
		{valid + ` {}`, "http://localhost:5173", 400},
	} {
		t.Run(tc.body+tc.origin, func(t *testing.T) {
			resolver := contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) {
				t.Fatal("Auth called before Origin/JSON validation")
				return domain.VerifiedContext{}, nil
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/analyses", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", tc.origin)
			recorder := httptest.NewRecorder()
			NewAnalysisHandler(nil, resolver, analysisScenarios{}, "http://localhost:5173").ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCreateAnalysisPassesOnlyVerifiedContextAndReturnsAccepted(t *testing.T) {
	id, requestID := domain.NewID(), domain.NewID()
	service := analysisScenarios{create: func(_ context.Context, command analysis.CreateAnalysisCommand) (domain.Analysis, error) {
		if command.Request.RequestID != requestID || command.Context.GuestSessionID == nil {
			t.Fatal("lost submission ID or verified identity")
		}
		return domain.Analysis{ID: id, Status: "queued", Repository: domain.RepositoryMetadata{Platform: "github", GitHubID: "42"}}, nil
	}}
	resolver := contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) { return activeGuestContext(), nil })
	request := httptest.NewRequest(http.MethodPost, "/api/v1/analyses", strings.NewReader(`{"request_id":"`+requestID+`","repository_url":"https://github.com/owner/repo"}`))
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: "opaque"})
	recorder := httptest.NewRecorder()
	NewAnalysisHandler(nil, resolver, service, "http://localhost:5173").ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"analysis_id":"`+id+`"`) || !strings.Contains(recorder.Body.String(), `"report_id":null`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

type combinedSessions struct{ guest apidomain.SessionInfo }

func (s combinedSessions) GetSessionContext(context.Context, apidomain.SessionTokens) (apidomain.SessionContext, error) {
	return apidomain.SessionContext{Guest: &s.guest}, nil
}

func (s combinedSessions) ContinueGuest(context.Context, apidomain.SessionTokens) (apidomain.IssuedSession, bool, error) {
	return apidomain.IssuedSession{Token: "opaque", Session: s.guest}, false, nil
}

func TestAnalysisRoutesPreserveMainSessionRoutes(t *testing.T) {
	id, requestID := domain.NewID(), domain.NewID()
	service := analysisScenarios{
		create: func(context.Context, analysis.CreateAnalysisCommand) (domain.Analysis, error) {
			return domain.Analysis{ID: id, Status: "queued"}, nil
		},
		get: func(context.Context, analysis.GetAnalysisQuery) (domain.Analysis, error) {
			return domain.Analysis{ID: id, Status: "queued", CreatedAt: time.Now().UTC()}, nil
		},
	}
	resolver := contextResolverFunc(func(context.Context, SessionTokens) (domain.VerifiedContext, error) {
		return activeGuestContext(), nil
	})
	handler := NewHandler(nil, resolver, Options{
		Sessions:     combinedSessions{guest: apidomain.SessionInfo{Kind: "guest", ExpiresAt: time.Now().Add(time.Hour)}},
		CookieSecure: false, PublicOrigin: "http://localhost:3000", Analyses: service,
	})
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/v1/auth/session", "", http.StatusOK},
		{"POST", "/api/v1/auth/guest-session", "", http.StatusOK},
		{"POST", "/api/v1/analyses", `{"request_id":"` + requestID + `","repository_url":"https://github.com/owner/repo"}`, http.StatusAccepted},
		{"GET", "/api/v1/analyses/" + id, "", http.StatusOK},
	} {
		t.Run(tc.path+tc.method, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Origin", "http://localhost:3000")
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: "opaque"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if tc.path == "/api/v1/auth/guest-session" {
				cookies := recorder.Result().Cookies()
				if len(cookies) != 1 || cookies[0].Secure || !cookies[0].HttpOnly {
					t.Fatal("main session cookie configuration was lost")
				}
			}
		})
	}
}
