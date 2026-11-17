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
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)



type analysisStore struct {
	submissions map[string]domain.Submission
	analyses    map[string]domain.Analysis
}

func newAnalysisStore() *analysisStore {
	return &analysisStore{
		submissions: make(map[string]domain.Submission),
		analyses:    make(map[string]domain.Analysis),
	}
}

var _ analysis.Store = (*analysisStore)(nil)

func (s *analysisStore) ReserveSubmission(
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

func (s *analysisStore) AcceptSubmission(
	ctx context.Context,
	a domain.Analysis,
) (domain.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.Analysis{}, err
	}

	key := a.Owner.ID + "/" + a.RequestID
	submission, ok := s.submissions[key]
	if !ok || submission.ID != a.SubmissionID {
		return domain.Analysis{}, domain.ErrNotFound
	}

	submission.AnalysisID = &a.ID
	submission.State = "accepted"
	s.submissions[key] = submission
	s.analyses[a.ID] = a

	return a, nil
}

func (s *analysisStore) RejectSubmission(
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

func (s *analysisStore) Get(
	_ context.Context,
	analysisID string,
) (domain.Analysis, error) {
	a, ok := s.analyses[analysisID]
	if !ok {
		return domain.Analysis{}, domain.ErrNotFound
	}

	return a, nil
}

type staticAnalysisSource struct{}

func (staticAnalysisSource) LookupForAnalysis(
	_ context.Context,
	addr domain.RepositoryURL,
) (domain.RepositoryMetadata, error) {
	return domain.RepositoryMetadata{
		Platform:     "github",
		GitHubID:      "42",
		CanonicalURL: addr.CanonicalURL(),
		Owner:        addr.Owner,
		Name:         addr.Name,
	}, nil
}

func TestAnalysisStatusRequiresOwnership(t *testing.T) {
	store := newMemoryAuthStore()
	var clockMu sync.Mutex
	offset := time.Duration(0)
	now := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return time.Now().UTC().Add(offset)
	}
	hasher, err := authpassword.NewArgon2id(authpassword.Params{HashMem: authpassword.MinMemoryKiB, HashTime: 1, HashThreads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	service := authapp.NewService(store, store, store, authtoken.NewGenerator(), now,
		authdomain.Options{GuestSessionTTL: 24 * time.Hour, AccountSessionTTL: 168 * time.Hour}, hasher)

	listener := bufconn.Listen(1 << 20)
	rpc := grpc.NewServer(grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
	authv1.RegisterAuthServiceServer(rpc, authgrpc.NewServer(service))
	go func() { _ = rpc.Serve(listener) }()
	defer rpc.Stop()
	conn, err := grpc.NewClient("passthrough:///auth",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	sessions := session.NewService(authclient.NewClient(conn, 5*time.Second))
	validator := &recordingValidator{inner: repository.NewValidator(&staticSource{}, time.Second)}

	// Version is used for versioning of scoring 
	// for test analyze it is not neseccary to be correct
	versions := domain.Versions{
	Schema:                "schema",
	Methodology:           "method",
	ScoringConfig:         "score",
	CollectorProfile:      "profile",
	SecretRules:           "secrets",
	RecommendationCatalog: "catalog",
	ExplanationSchema:     "explanation",
	Prompt:                "prompt",
}

	analysisStore := newAnalysisStore()
	
	// creating real analyses service, to create analyze_id 
	// for A and check it for B
	analyses := analysis.NewService(
	analysisStore,
	staticAnalysisSource{},
	versions,
	time.Second,
	)


	handler := NewHandler(validator, sessions, Options{Sessions: sessions, CookieSecure: true, PublicOrigin: testOrigin, Analyses: analyses})

	request := func(method, path, body, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	errorCode := func(w *httptest.ResponseRecorder) string {
		var body errorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body.Error.Code
	}
	credentialsA := `{"login":" FlameX ","password":"correct horse battery"}`

	credentialsB := `{"login":" FlameX1 ", "password":"correct horse battery1"}`
	// CSRF: state-changing auth requests need the exact configured Origin.
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/logout"} {
		if w := request("POST", path, credentialsA, "https://evil.example"); w.Code != 403 || errorCode(w) != "CSRF_REJECTED" {
			t.Fatalf("%s from a foreign origin: %d %s", path, w.Code, w.Body)
		}
	}
	if len(store.principals) != 0 {
		t.Fatal("rejected request changed state")
	}
	// register two accounts A and B
	registeredA := request("POST", "/api/v1/auth/register", credentialsA, testOrigin)
	if registeredA.Code != 201 || strings.TrimSpace(registeredA.Body.String()) != `{"login":"flamex","message":"Аккаунт создан. Войдите в систему"}` {
		t.Fatalf("register: %d %s", registeredA.Code, registeredA.Body)
	}
	if len(registeredA.Result().Cookies()) != 0 {
		t.Fatal("registration must not sign in")
	}

	registeredB := request("POST", "/api/v1/auth/register", credentialsB, testOrigin)
	if registeredB.Code != 201 || strings.TrimSpace(registeredB.Body.String()) != `{"login":"flamex1","message":"Аккаунт создан. Войдите в систему"}` {
		t.Fatalf("register: %d %s", registeredB.Code, registeredB.Body)
	}
	if len(registeredB.Result().Cookies()) != 0 {
		t.Fatal("registration must not sign in")
	}
	
	// login two accounts A and B
	// and save their cookies
	loggedInA := request("POST", "/api/v1/auth/login", `{"login":"flamex","password":"correct horse battery"}`, testOrigin)
	if loggedInA.Code != 200 || !strings.Contains(loggedInA.Body.String(), `"kind":"account","login":"flamex"`) {
		t.Fatalf("login: %d %s", loggedInA.Code, loggedInA.Body)
	}
	cookiesA := loggedInA.Result().Cookies()
	if len(cookiesA) != 1 || cookiesA[0].Name != "repopulse_session" {
		t.Fatalf("login cookies = %+v; only the account cookie may be set", cookiesA)
	}
	accountA := cookiesA[0]
	weekSeconds := int((168 * time.Hour).Seconds())
	if !accountA.HttpOnly || !accountA.Secure || accountA.SameSite != http.SameSiteLaxMode || accountA.Path != "/api/v1" ||
		accountA.MaxAge < weekSeconds-60 || accountA.MaxAge > weekSeconds {
		t.Fatalf("bad account cookie: %+v", accountA)
	}
	if strings.Contains(loggedInA.Body.String(), accountA.Value) {
		t.Fatal("token exposed in JSON")
	}


	loggedInB := request("POST", "/api/v1/auth/login", `{"login":"flamex1","password":"correct horse battery1"}`, testOrigin)
	if loggedInB.Code != 200 || !strings.Contains(loggedInB.Body.String(), `"kind":"account","login":"flamex1"`) {
		t.Fatalf("login: %d %s", loggedInB.Code, loggedInB.Body)
	}
	cookiesB := loggedInB.Result().Cookies()
	if len(cookiesB) != 1 || cookiesB[0].Name != "repopulse_session" {
		t.Fatalf("login cookies = %+v; only the account cookie may be set", cookiesB)
	}
	accountB := cookiesB[0]
	if !accountA.HttpOnly || !accountB.Secure || accountB.SameSite != http.SameSiteLaxMode || accountB.Path != "/api/v1" ||
		accountB.MaxAge < weekSeconds-60 || accountB.MaxAge > weekSeconds {
		t.Fatalf("bad account cookie: %+v", accountB)
	}
	if strings.Contains(loggedInB.Body.String(), accountB.Value) {
		t.Fatal("token exposed in JSON")
	}
	body := fmt.Sprintf(
	`{"request_id":%q,"repository_url":"https://github.com/owner/repo"}`,
	uuid.NewString(),
	)
	created := request("POST", "/api/v1/analyses", body, testOrigin, accountA)

	if created.Code != http.StatusAccepted {
		t.Fatalf("create analysis: got HTTP %d, body=%s", created.Code, created.Body)
	}

	var result struct {
		AnalysisID string `json:"analysis_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if result.AnalysisID == "" {
		t.Fatal("create response has no analysis_id")
	}

	path := "/api/v1/analyses/" + result.AnalysisID

	asA := request(http.MethodGet, path, "", "", accountA)
	if asA.Code != http.StatusOK {
		t.Fatalf("owner reads analysis: got %d, want 200, body=%s", asA.Code, asA.Body)
	}

	asB := request(http.MethodGet, path, "", "", accountB)
	if asB.Code != http.StatusNotFound || errorCode(asB) != "ANALYSIS_NOT_FOUND" {
		t.Fatalf("other account reads analysis: got %d, want 404 ANALYSIS_NOT_FOUND, body=%s",
		asB.Code, asB.Body)
	}
}
