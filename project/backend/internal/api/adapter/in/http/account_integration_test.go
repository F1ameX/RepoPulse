package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	analyzerdomain "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/application/session"
	authgrpc "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	authpassword "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/password"
	authtoken "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/token"
	authapp "github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
)

// memoryAuthStore implements the Auth storage ports in memory with the same
// rules as PostgreSQL: unique login, sessions by hash, idempotent revocation.
type memoryAuthStore struct {
	mu         sync.Mutex
	principals map[string]authdomain.Principal // by ID
	sessions   map[string]authdomain.Session   // by token hash
}

func newMemoryAuthStore() *memoryAuthStore {
	return &memoryAuthStore{principals: map[string]authdomain.Principal{}, sessions: map[string]authdomain.Session{}}
}

func (s *memoryAuthStore) CreateAccount(_ context.Context, login, hash string, createdAt time.Time) (authdomain.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.principals {
		if p.Login == login {
			return authdomain.Principal{}, authdomain.ErrLoginTaken
		}
	}
	p := authdomain.Principal{ID: uuid.NewString(), Kind: authdomain.KindAccount, Login: login, PasswordHash: hash, CreatedAt: createdAt}
	s.principals[p.ID] = p
	return p, nil
}

func (s *memoryAuthStore) FindAccountByLogin(_ context.Context, login string) (authdomain.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.principals {
		if p.Kind == authdomain.KindAccount && p.Login == login {
			return p, nil
		}
	}
	return authdomain.Principal{}, authdomain.ErrInvalidCredentials
}

func (s *memoryAuthStore) FindAccountByID(_ context.Context, id string) (authdomain.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.principals[id]; ok && p.Kind == authdomain.KindAccount {
		return p, nil
	}
	return authdomain.Principal{}, authdomain.ErrPrincipalNotFound
}

func (s *memoryAuthStore) CreateSession(_ context.Context, session authdomain.Session) (authdomain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.ID = uuid.NewString()
	s.sessions[session.TokenHash] = session
	return session, nil
}

func (s *memoryAuthStore) CreateGuestSession(_ context.Context, hash string, issuedAt, expiresAt time.Time) (authdomain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	guest := authdomain.Principal{ID: uuid.NewString(), Kind: authdomain.KindGuest, CreatedAt: issuedAt}
	s.principals[guest.ID] = guest
	session := authdomain.Session{ID: uuid.NewString(), PrincipalID: guest.ID, Kind: authdomain.KindGuest,
		TokenHash: hash, IssuedAt: issuedAt, ExpiresAt: expiresAt}
	s.sessions[hash] = session
	return session, nil
}

func (s *memoryAuthStore) FindSessionByTokenHash(_ context.Context, hash string) (authdomain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[hash]; ok {
		return session, nil
	}
	return authdomain.Session{}, authdomain.ErrSessionNotFound
}

func (s *memoryAuthStore) RevokeSession(_ context.Context, id string, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, session := range s.sessions {
		if session.ID == id && session.RevokedAt == nil {
			session.RevokedAt = &at
			s.sessions[hash] = session
			return true, nil
		}
	}
	return false, nil
}

// staticSource answers every lookup with a public repository.
type staticSource struct{}

func (s *staticSource) Lookup(_ context.Context, address analyzerdomain.RepositoryURL) (analyzerdomain.ValidatedRepository, error) {
	return analyzerdomain.ValidatedRepository{Repository: analyzerdomain.Repository{GitHubID: "42",
		CanonicalURL: address.CanonicalURL(), Owner: address.Owner, Name: address.Name}}, nil
}

type recordingValidator struct {
	inner    *repository.Validator
	contexts []analyzerdomain.VerifiedContext
}

func (v *recordingValidator) ValidateRepository(ctx context.Context, cmd repository.ValidateRepositoryCommand) (analyzerdomain.ValidatedRepository, error) {
	v.contexts = append(v.contexts, cmd.Context)
	return v.inner.ValidateRepository(ctx, cmd)
}

const testOrigin = "https://repopulse.example"

// TestAccountHTTPGRPCFlow walks FR-27 acceptance through HTTP, the Gateway Auth
// client, gRPC and the real Auth scenarios with Argon2id.
func TestAccountHTTPGRPCFlow(t *testing.T) {
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
	handler := NewHandler(validator, sessions, Options{Sessions: sessions, CookieSecure: true, PublicOrigin: testOrigin})

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
	credentials := `{"login":" FlameX ","password":"correct horse battery"}`

	// CSRF: state-changing auth requests need the exact configured Origin.
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/logout"} {
		if w := request("POST", path, credentials, "https://evil.example"); w.Code != 403 || errorCode(w) != "CSRF_REJECTED" {
			t.Fatalf("%s from a foreign origin: %d %s", path, w.Code, w.Body)
		}
	}
	if len(store.principals) != 0 {
		t.Fatal("rejected request changed state")
	}

	// Registration: 201 with the normalized login, no cookie, no automatic sign-in.
	registered := request("POST", "/api/v1/auth/register", credentials, testOrigin)
	if registered.Code != 201 || strings.TrimSpace(registered.Body.String()) != `{"login":"flamex","message":"Аккаунт создан. Войдите в систему"}` {
		t.Fatalf("register: %d %s", registered.Code, registered.Body)
	}
	if len(registered.Result().Cookies()) != 0 {
		t.Fatal("registration must not sign in")
	}
	for _, again := range []string{`{"login":"flamex","password":"correct horse battery"}`, `{"login":"FLAMEX","password":"another password!!"}`} {
		if w := request("POST", "/api/v1/auth/register", again, testOrigin); w.Code != 409 || errorCode(w) != "LOGIN_TAKEN" {
			t.Fatalf("duplicate login: %d %s", w.Code, w.Body)
		}
	}
	invalid := request("POST", "/api/v1/auth/register", `{"login":"","password":"short"}`, testOrigin)
	var invalidBody errorResponse
	if err := json.Unmarshal(invalid.Body.Bytes(), &invalidBody); err != nil || invalid.Code != 400 ||
		invalidBody.Error.Code != "VALIDATION_FAILED" || len(invalidBody.Error.FieldErrors) != 2 ||
		invalidBody.Error.FieldErrors[0] != (fieldError{Field: "login", Code: "REQUIRED", Message: "Укажите логин"}) {
		t.Fatalf("validation: %d %s", invalid.Code, invalid.Body)
	}
	if strings.Contains(invalid.Body.String(), "short") {
		t.Fatal("the password is repeated in the error")
	}
	for _, malformed := range []string{`{"login":"flamex"}`, `{"login":"a","password":"b","extra":1}`,
		`{"login":"a","login":"b","password":"c"}`, `{"Login":"a","password":"b"}`, `{"login":null,"password":"b"}`, `[]`, `{} {}`} {
		if w := request("POST", "/api/v1/auth/register", malformed, testOrigin); w.Code != 400 || errorCode(w) != "VALIDATION_FAILED" {
			t.Fatalf("malformed %s: %d %s", malformed, w.Code, w.Body)
		}
	}
	if len(store.principals) != 1 {
		t.Fatalf("accounts = %d; rejected registrations must not create accounts", len(store.principals))
	}

	// Unknown login and wrong password: the same 401 and no session.
	for _, wrong := range []string{`{"login":"flamex","password":"wrong password value"}`, `{"login":"nosuchuser","password":"correct horse battery"}`} {
		w := request("POST", "/api/v1/auth/login", wrong, testOrigin)
		if w.Code != 401 || errorCode(w) != "INVALID_CREDENTIALS" || !strings.Contains(w.Body.String(), "Неверный логин или пароль") || len(w.Result().Cookies()) != 0 {
			t.Fatalf("wrong credentials: %d %s", w.Code, w.Body)
		}
	}

	// A guest exists before the login; the login must keep it.
	guestResponse := request("POST", "/api/v1/auth/guest-session", "", testOrigin)
	guest := guestResponse.Result().Cookies()[0]

	// Successful login: persistent account cookie until expires_at, guest cookie untouched.
	loggedIn := request("POST", "/api/v1/auth/login", `{"login":"FLAMEX","password":"correct horse battery"}`, testOrigin, guest)
	if loggedIn.Code != 200 || !strings.Contains(loggedIn.Body.String(), `"kind":"account","login":"flamex"`) {
		t.Fatalf("login: %d %s", loggedIn.Code, loggedIn.Body)
	}
	cookies := loggedIn.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "repopulse_session" {
		t.Fatalf("login cookies = %+v; only the account cookie may be set", cookies)
	}
	account := cookies[0]
	weekSeconds := int((168 * time.Hour).Seconds())
	if !account.HttpOnly || !account.Secure || account.SameSite != http.SameSiteLaxMode || account.Path != "/api/v1" ||
		account.MaxAge < weekSeconds-60 || account.MaxAge > weekSeconds {
		t.Fatalf("bad account cookie: %+v", account)
	}
	if strings.Contains(loggedIn.Body.String(), account.Value) {
		t.Fatal("token exposed in JSON")
	}

	// Both contexts are verified; the account is presented to the owner choice.
	contextResponse := request("GET", "/api/v1/auth/session", "", "", account, guest)
	if contextResponse.Code != 200 || !strings.Contains(contextResponse.Body.String(), `"account":{"kind":"account","login":"flamex"`) ||
		!strings.Contains(contextResponse.Body.String(), `"guest":{"kind":"guest","login":null`) {
		t.Fatalf("session context: %d %s", contextResponse.Code, contextResponse.Body)
	}
	valid := request("POST", "/api/v1/repositories/validate", `{"repository_url":"https://github.com/owner/repo"}`, testOrigin, account, guest)
	if valid.Code != 200 {
		t.Fatalf("validate with account: %d %s", valid.Code, valid.Body)
	}
	last := validator.contexts[len(validator.contexts)-1]
	if last.UserID == nil || last.AccountExpiresAt == nil || last.GuestSessionID == nil || *last.UserID == *last.GuestSessionID {
		t.Fatalf("verified context = %+v; want both the account and the original guest", last)
	}

	// A second, independent login of the same account.
	second := request("POST", "/api/v1/auth/login", credentials, testOrigin).Result().Cookies()[0]

	// Logout: 204, account cookie deleted with the same attributes, guest cookie kept.
	out := request("POST", "/api/v1/auth/logout", "", testOrigin, account, guest)
	if out.Code != 204 || out.Body.Len() != 0 {
		t.Fatalf("logout: %d %s", out.Code, out.Body)
	}
	deleted := out.Result().Cookies()
	if len(deleted) != 1 || deleted[0].Name != "repopulse_session" || deleted[0].MaxAge >= 0 || deleted[0].Path != "/api/v1" ||
		!deleted[0].HttpOnly || !deleted[0].Secure || deleted[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("logout cookies = %+v; only the account cookie may be deleted", deleted)
	}
	// Reusing the revoked session is rejected and never falls back to the guest.
	if w := request("GET", "/api/v1/auth/session", "", "", account, guest); w.Code != 401 || errorCode(w) != "SESSION_EXPIRED" {
		t.Fatalf("revoked session reused: %d %s", w.Code, w.Body)
	}
	// Logout is idempotent, also without any cookie.
	for _, cookies := range [][]*http.Cookie{{account}, {}} {
		if w := request("POST", "/api/v1/auth/logout", "", testOrigin, cookies...); w.Code != 204 {
			t.Fatalf("repeated logout: %d %s", w.Code, w.Body)
		}
	}
	// The independent session and the guest are still valid.
	if w := request("GET", "/api/v1/auth/session", "", "", second, guest); w.Code != 200 || !strings.Contains(w.Body.String(), `"login":"flamex"`) ||
		!strings.Contains(w.Body.String(), `"kind":"guest"`) {
		t.Fatalf("independent sessions after logout: %d %s", w.Code, w.Body)
	}

	// The 7-day boundary: valid just before, invalid exactly at it.
	expiresAt := store.sessions[authtoken.NewGenerator().Hash(second.Value)].ExpiresAt
	setNow := func(at time.Time) {
		clockMu.Lock()
		offset = time.Until(at)
		clockMu.Unlock()
	}
	setNow(expiresAt.Add(-time.Second))
	if w := request("GET", "/api/v1/auth/session", "", "", second); w.Code != 200 || !strings.Contains(w.Body.String(), `"login":"flamex"`) {
		t.Fatalf("before expiry: %d %s", w.Code, w.Body)
	}
	setNow(expiresAt.Add(time.Millisecond))
	if w := request("GET", "/api/v1/auth/session", "", "", second); w.Code != 401 || errorCode(w) != "SESSION_EXPIRED" {
		t.Fatalf("at expiry: %d %s", w.Code, w.Body)
	}

	// An Auth outage is a separate error, never "invalid credentials" or a success.
	rpc.Stop()
	if w := request("POST", "/api/v1/auth/login", credentials, testOrigin); w.Code != 503 || errorCode(w) != "AUTH_UNAVAILABLE" {
		t.Fatalf("login during outage: %d %s", w.Code, w.Body)
	}
	if w := request("POST", "/api/v1/auth/logout", "", testOrigin, second); w.Code != 503 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("unconfirmed logout must keep the cookie: %d %+v", w.Code, w.Result().Cookies())
	}
}
