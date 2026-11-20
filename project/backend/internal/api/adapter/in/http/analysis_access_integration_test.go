package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/application/session"
	authgrpc "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	authpassword "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/password"
	authtoken "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/token"
	authapp "github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	reportapp "github.com/F1ameX/RepoPulse/project/backend/internal/report/application/report"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type analysisAccessStore struct {
	submissions map[string]domain.Submission
	analyses    map[string]domain.Analysis
}

func newAnalysisAccessStore() *analysisAccessStore {
	return &analysisAccessStore{
		submissions: make(map[string]domain.Submission),
		analyses:    make(map[string]domain.Analysis),
	}
}

var _ analysis.Store = (*analysisAccessStore)(nil)

func (s *analysisAccessStore) ReserveSubmission(
	_ context.Context,
	submission domain.Submission,
) (domain.Submission, bool, error) {
	key := submission.Owner.ID + "/" + submission.RequestID
	if existing, ok := s.submissions[key]; ok {
		return existing, false, nil
	}

	s.submissions[key] = submission
	return submission, true, nil
}

func (s *analysisAccessStore) AcceptSubmission(
	ctx context.Context,
	record domain.Analysis,
) (domain.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.Analysis{}, err
	}

	key := record.Owner.ID + "/" + record.RequestID
	submission, ok := s.submissions[key]
	if !ok || submission.ID != record.SubmissionID {
		return domain.Analysis{}, domain.ErrNotFound
	}

	submission.AnalysisID = &record.ID
	submission.State = "accepted"
	s.submissions[key] = submission
	s.analyses[record.ID] = record

	return record, nil
}

func (s *analysisAccessStore) RejectSubmission(
	_ context.Context,
	submissionID string,
	failure domain.SubmissionFailure,
) error {
	for key, submission := range s.submissions {
		if submission.ID == submissionID {
			submission.State = "rejected"
			submission.Failure = &failure
			s.submissions[key] = submission
			return nil
		}
	}

	return domain.ErrNotFound
}

func (s *analysisAccessStore) Get(
	_ context.Context,
	analysisID string,
) (domain.Analysis, error) {
	record, ok := s.analyses[analysisID]
	if !ok {
		return domain.Analysis{}, domain.ErrNotFound
	}

	return record, nil
}

type staticAnalysisSource struct{}

func (staticAnalysisSource) LookupForAnalysis(
	_ context.Context,
	address domain.RepositoryURL,
) (domain.RepositoryMetadata, error) {
	return domain.RepositoryMetadata{
		Platform:     "github",
		GitHubID:     "42",
		CanonicalURL: address.CanonicalURL(),
		Owner:        address.Owner,
		Name:         address.Name,
	}, nil
}

// analysisAccessFixture runs real Auth/Analyzer/Report scenarios with in-memory
// repositories and an in-process Auth gRPC server.
type analysisAccessFixture struct {
	handler       http.Handler
	authStore     *memoryAuthStore
	analysisStore *analysisAccessStore
	reports       *guestAccessReportStore
	clockMu       sync.Mutex
	currentTime   time.Time
}

func newAnalysisAccessFixture(t *testing.T) *analysisAccessFixture {
	t.Helper()
	fixture := &analysisAccessFixture{
		authStore:     newMemoryAuthStore(),
		analysisStore: newAnalysisAccessStore(),
		reports:       &guestAccessReportStore{},
		currentTime:   time.Now().UTC().Truncate(time.Microsecond),
	}
	hasher, err := authpassword.NewArgon2id(authpassword.Params{
		HashMem: authpassword.MinMemoryKiB, HashTime: 1, HashThreads: 1, KeyLen: 32, SaltLen: 16,
	})
	if err != nil {
		t.Fatal(err)
	}
	authService := authapp.NewService(fixture.authStore, fixture.authStore, fixture.authStore,
		authtoken.NewGenerator(), fixture.now,
		authdomain.Options{GuestSessionTTL: 24 * time.Hour, AccountSessionTTL: 168 * time.Hour}, hasher)

	listener := bufconn.Listen(1 << 20)
	t.Cleanup(func() { _ = listener.Close() })
	authServer := grpc.NewServer(grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
	authv1.RegisterAuthServiceServer(authServer, authgrpc.NewServer(authService))
	go func() { _ = authServer.Serve(listener) }()
	t.Cleanup(authServer.Stop)

	authConnection, err := grpc.NewClient("passthrough:///auth",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authConnection.Close() })

	sessions := session.NewService(authclient.NewClient(authConnection, 5*time.Second))
	validator := &recordingValidator{inner: repository.NewValidator(&staticSource{}, time.Second)}
	// Access checks need version identifiers, but do not calculate scores.
	versions := domain.Versions{
		Schema: "schema", Methodology: "method", ScoringConfig: "score", CollectorProfile: "profile",
		SecretRules: "secrets", RecommendationCatalog: "catalog", ExplanationSchema: "explanation", Prompt: "prompt",
	}
	analyses := analysis.NewService(fixture.analysisStore, staticAnalysisSource{}, versions, time.Second)
	fixture.handler = NewHandler(validator, sessions, Options{
		Sessions: sessions, CookieSecure: true, PublicOrigin: testOrigin, Analyses: analyses,
		Reports: reportapp.NewService(fixture.reports),
	})
	return fixture
}

func (f *analysisAccessFixture) now() time.Time {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	return f.currentTime
}

func (f *analysisAccessFixture) setNow(at time.Time) {
	f.clockMu.Lock()
	defer f.clockMu.Unlock()
	f.currentTime = at
}

func (f *analysisAccessFixture) request(t *testing.T, method, path, body, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	return response
}

func requireAccessResponse(t *testing.T, response *httptest.ResponseRecorder, status int, errorCode string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status: got %d, want %d, body=%s", response.Code, status, response.Body)
	}
	if errorCode == "" {
		return
	}
	var failure errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if failure.Error.Code != errorCode {
		t.Fatalf("error code: got %q, want %q, body=%s", failure.Error.Code, errorCode, response.Body)
	}
}

func accessCredentials(login, password string) string {
	return fmt.Sprintf(`{"login":%q,"password":%q}`, login, password)
}

func (f *analysisAccessFixture) registerAccount(t *testing.T, login, password string) {
	t.Helper()
	response := f.request(t, http.MethodPost, "/api/v1/auth/register", accessCredentials(login, password), testOrigin)
	requireAccessResponse(t, response, http.StatusCreated, "")
	normalizedLogin := strings.ToLower(strings.Trim(login, " "))
	expected := fmt.Sprintf(`{"login":%q,"message":"Аккаунт создан. Войдите в систему"}`, normalizedLogin)
	if strings.TrimSpace(response.Body.String()) != expected {
		t.Fatalf("register response: %s", response.Body)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("registration must not sign in")
	}
}

type accessAccountSession struct {
	cookie    *http.Cookie
	expiresAt time.Time
}

func (f *analysisAccessFixture) login(t *testing.T, login, password string) accessAccountSession {
	t.Helper()
	response := f.request(t, http.MethodPost, "/api/v1/auth/login", accessCredentials(login, password), testOrigin)
	requireAccessResponse(t, response, http.StatusOK, "")
	if !strings.Contains(response.Body.String(), fmt.Sprintf(`"kind":"account","login":%q`, login)) {
		t.Fatalf("login response: %s", response.Body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "repopulse_session" {
		t.Fatalf("expected one account cookie, got %+v", cookies)
	}
	cookie := cookies[0]
	weekSeconds := int((168 * time.Hour).Seconds())
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/v1" ||
		cookie.MaxAge < weekSeconds-60 || cookie.MaxAge > weekSeconds {
		t.Fatalf("bad account cookie: %+v", cookie)
	}
	if strings.Contains(response.Body.String(), cookie.Value) {
		t.Fatal("token exposed in JSON")
	}

	var sessionInfo struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &sessionInfo); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if sessionInfo.ExpiresAt.IsZero() {
		t.Fatal("login response has no expires_at")
	}
	return accessAccountSession{cookie: cookie, expiresAt: sessionInfo.ExpiresAt}
}

func (f *analysisAccessFixture) createGuestSession(t *testing.T) *http.Cookie {
	t.Helper()
	response := f.request(t, http.MethodPost, "/api/v1/auth/guest-session", "", testOrigin)
	requireAccessResponse(t, response, http.StatusCreated, "")
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "repopulse_guest" {
		t.Fatal("expected one repopulse_guest cookie")
	}
	var sessionInfo struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &sessionInfo); err != nil {
		t.Fatalf("decode guest session: %v", err)
	}
	if sessionInfo.ExpiresAt.IsZero() {
		t.Fatal("guest response has no expires_at")
	}
	return cookies[0]
}

func accessAnalysisID(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var status struct {
		AnalysisID string `json:"analysis_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode analysis response: %v", err)
	}
	if status.AnalysisID == "" {
		t.Fatal("response has no analysis_id")
	}
	return status.AnalysisID
}

func (f *analysisAccessFixture) createAnalysis(t *testing.T, cookies ...*http.Cookie) string {
	t.Helper()
	body := fmt.Sprintf(`{"request_id":%q,"repository_url":"https://github.com/owner/repo"}`, uuid.NewString())
	response := f.request(t, http.MethodPost, "/api/v1/analyses", body, testOrigin, cookies...)
	requireAccessResponse(t, response, http.StatusAccepted, "")
	return accessAnalysisID(t, response)
}

const accessTestPassword = "correct horse battery"

func TestAnalysisStatusRequiresOwnership(t *testing.T) {
	fixture := newAnalysisAccessFixture(t)
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/logout"} {
		response := fixture.request(t, http.MethodPost, path, accessCredentials(" FlameX ", accessTestPassword), "https://evil.example")
		requireAccessResponse(t, response, http.StatusForbidden, "CSRF_REJECTED")
	}
	if len(fixture.authStore.principals) != 0 {
		t.Fatal("rejected request changed state")
	}

	fixture.registerAccount(t, " FlameX ", accessTestPassword)
	fixture.registerAccount(t, " FlameX1 ", accessTestPassword+"1")
	ownerSession := fixture.login(t, "flamex", accessTestPassword)
	otherSession := fixture.login(t, "flamex1", accessTestPassword+"1")
	analysisID := fixture.createAnalysis(t, ownerSession.cookie)
	analysisPath := "/api/v1/analyses/" + analysisID

	ownerResponse := fixture.request(t, http.MethodGet, analysisPath, "", "", ownerSession.cookie)
	requireAccessResponse(t, ownerResponse, http.StatusOK, "")
	for _, testCase := range []struct{ name, path string }{
		{"foreign account with correct analysis_id", analysisPath},
		{"foreign account with incorrect analysis_id", "/api/v1/analyses/" + uuid.NewString()},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := fixture.request(t, http.MethodGet, testCase.path, "", "", otherSession.cookie)
			requireAccessResponse(t, response, http.StatusNotFound, "ANALYSIS_NOT_FOUND")
		})
	}
}

func TestAnalysisStatusAccessRestoredAfterRelogin(t *testing.T) {
	fixture := newAnalysisAccessFixture(t)
	fixture.registerAccount(t, " FlameX ", accessTestPassword)
	originalSession := fixture.login(t, "flamex", accessTestPassword)
	analysisID := fixture.createAnalysis(t, originalSession.cookie)
	analysisPath := "/api/v1/analyses/" + analysisID

	ownerResponse := fixture.request(t, http.MethodGet, analysisPath, "", "", originalSession.cookie)
	requireAccessResponse(t, ownerResponse, http.StatusOK, "")

	logoutResponse := fixture.request(t, http.MethodPost, "/api/v1/auth/logout", "", testOrigin, originalSession.cookie)
	requireAccessResponse(t, logoutResponse, http.StatusNoContent, "")
	if logoutResponse.Body.Len() != 0 {
		t.Fatalf("logout body: %s", logoutResponse.Body)
	}
	deletedCookies := logoutResponse.Result().Cookies()
	if len(deletedCookies) != 1 || deletedCookies[0].Name != "repopulse_session" || deletedCookies[0].MaxAge >= 0 || deletedCookies[0].Path != "/api/v1" ||
		!deletedCookies[0].HttpOnly || !deletedCookies[0].Secure || deletedCookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("logout cookies = %+v; only the account cookie may be deleted", deletedCookies)
	}

	revokedResponse := fixture.request(t, http.MethodGet, analysisPath, "", "", originalSession.cookie)
	requireAccessResponse(t, revokedResponse, http.StatusUnauthorized, "SESSION_EXPIRED")
	renewedSession := fixture.login(t, "flamex", accessTestPassword)
	restoredResponse := fixture.request(t, http.MethodGet, analysisPath, "", "", renewedSession.cookie)
	requireAccessResponse(t, restoredResponse, http.StatusOK, "")
}

func TestAnalysisStatusRejectsExpiredAccountSession(t *testing.T) {
	fixture := newAnalysisAccessFixture(t)
	fixture.registerAccount(t, " FlameX ", accessTestPassword)
	accountSession := fixture.login(t, "flamex", accessTestPassword)
	analysisID := fixture.createAnalysis(t, accountSession.cookie)
	analysisPath := "/api/v1/analyses/" + analysisID

	for _, testCase := range []struct {
		name       string
		offset     time.Duration
		wantStatus int
		wantCode   string
	}{
		{"before expiry", -time.Microsecond, http.StatusOK, ""},
		{"exactly at expiry", 0, http.StatusUnauthorized, "SESSION_EXPIRED"},
		{"after expiry", time.Microsecond, http.StatusUnauthorized, "SESSION_EXPIRED"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture.setNow(accountSession.expiresAt.Add(testCase.offset))
			response := fixture.request(t, http.MethodGet, analysisPath, "", "", accountSession.cookie)
			requireAccessResponse(t, response, testCase.wantStatus, testCase.wantCode)
		})
	}
}

func TestGuestAnalysisAccessWithAccountSession(t *testing.T) {
	fixture := newAnalysisAccessFixture(t)
	guestCookie := fixture.createGuestSession(t)
	guestAnalysisID := fixture.createAnalysis(t, guestCookie)
	guestAnalysisPath := "/api/v1/analyses/" + guestAnalysisID

	// Seed an existing guest report to distinguish denied access from a missing report.
	fixture.reports.report = domain.Report{ID: uuid.NewString(), AnalysisID: guestAnalysisID}
	fixture.reports.ownerID = fixture.analysisStore.analyses[guestAnalysisID].Owner.ID
	guestReportPath := "/api/v1/reports/" + fixture.reports.report.ID

	fixture.registerAccount(t, " FlameX ", accessTestPassword)
	accountSession := fixture.login(t, "flamex", accessTestPassword)

	guestAnalysisResponse := fixture.request(t, http.MethodGet, guestAnalysisPath, "", "", accountSession.cookie, guestCookie)
	requireAccessResponse(t, guestAnalysisResponse, http.StatusOK, "")
	if gotID := accessAnalysisID(t, guestAnalysisResponse); gotID != guestAnalysisID {
		t.Fatalf("guest analysis_id=%q, want %q", gotID, guestAnalysisID)
	}
	guestReportResponse := fixture.request(t, http.MethodGet, guestReportPath, "", "", accountSession.cookie, guestCookie)
	requireAccessResponse(t, guestReportResponse, http.StatusOK, "")

	accountOnlyAnalysisResponse := fixture.request(t, http.MethodGet, guestAnalysisPath, "", "", accountSession.cookie)
	requireAccessResponse(t, accountOnlyAnalysisResponse, http.StatusNotFound, "ANALYSIS_NOT_FOUND")
	accountOnlyReportResponse := fixture.request(t, http.MethodGet, guestReportPath, "", "", accountSession.cookie)
	requireAccessResponse(t, accountOnlyReportResponse, http.StatusNotFound, "REPORT_NOT_FOUND")

	accountAnalysisID := fixture.createAnalysis(t, accountSession.cookie, guestCookie)
	if accountAnalysisID == guestAnalysisID {
		t.Fatal("account and guest analysis IDs must differ")
	}
	accountAnalysisPath := "/api/v1/analyses/" + accountAnalysisID
	accountAnalysisResponse := fixture.request(t, http.MethodGet, accountAnalysisPath, "", "", accountSession.cookie)
	requireAccessResponse(t, accountAnalysisResponse, http.StatusOK, "")
	if gotID := accessAnalysisID(t, accountAnalysisResponse); gotID != accountAnalysisID {
		t.Fatalf("account analysis_id=%q, want %q", gotID, accountAnalysisID)
	}
	guestOnlyResponse := fixture.request(t, http.MethodGet, accountAnalysisPath, "", "", guestCookie)
	requireAccessResponse(t, guestOnlyResponse, http.StatusNotFound, "ANALYSIS_NOT_FOUND")

	if fixture.analysisStore.analyses[guestAnalysisID].Owner.ID != fixture.reports.ownerID {
		t.Fatal("guest analysis owner changed")
	}
}

// guestAccessReportStore supplies a report to the real Report ownership scenario.
type guestAccessReportStore struct {
	report  domain.Report
	ownerID string
}

func (s *guestAccessReportStore) Get(_ context.Context, id string) (domain.Report, string, error) {
	if s.report.ID != id {
		return domain.Report{}, "", domain.ErrNotFound
	}
	return s.report, s.ownerID, nil
}
