package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	analyzerdomain "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/application/session"
	authgrpc "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	authtoken "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/token"
	authapp "github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type sessionStore struct {
	authapp.TokenRepository
	sessions map[string]authdomain.Session
	created  int
}

func (s *sessionStore) CreateGuestSession(_ context.Context, hash string, issuedAt, expiresAt time.Time) (authdomain.Session, error) {
	s.created++
	result := authdomain.Session{ID: "session", PrincipalID: "guest-principal", Kind: authdomain.KindGuest, TokenHash: hash, IssuedAt: issuedAt, ExpiresAt: expiresAt}
	s.sessions[hash] = result
	return result, nil
}
func (s *sessionStore) FindSessionByTokenHash(_ context.Context, hash string) (authdomain.Session, error) {
	result, ok := s.sessions[hash]
	if !ok {
		return authdomain.Session{}, authdomain.ErrSessionNotFound
	}
	return result, nil
}

type publicSource struct{ calls int }

func (s *publicSource) Lookup(_ context.Context, address analyzerdomain.RepositoryURL) (analyzerdomain.ValidatedRepository, error) {
	s.calls++
	return analyzerdomain.ValidatedRepository{Repository: analyzerdomain.Repository{GitHubID: "42", CanonicalURL: address.CanonicalURL(), Owner: address.Owner, Name: address.Name}}, nil
}

func TestGuestHTTPGRPCValidationFlow(t *testing.T) {
	now := time.Now().UTC()
	store := &sessionStore{sessions: map[string]authdomain.Session{}}
	listener := bufconn.Listen(1 << 20)
	rpc := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcerr.UnaryServerInterceptor(), func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ids := metadata.ValueFromIncomingContext(ctx, grpcerr.RequestIDKey)
		if len(ids) != 1 || ids[0] == "" {
			t.Error("missing HTTP trace in Auth request")
		}
		return handler(ctx, req)
	}))
	authv1.RegisterAuthServiceServer(rpc, authgrpc.NewServer(authapp.NewService(store, nil, store, authtoken.NewGenerator(), func() time.Time { return now },
		authdomain.Options{GuestSessionTTL: 24 * time.Hour, AccountSessionTTL: 168 * time.Hour}, nil)))
	go func() { _ = rpc.Serve(listener) }()
	defer rpc.Stop()
	conn, err := grpc.NewClient("passthrough:///auth", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sessions := session.NewService(authclient.NewClient(conn, time.Second))
	source := &publicSource{}
	handler := NewHandler(repository.NewValidator(source, time.Second), sessions, Options{Sessions: sessions, CookieSecure: true, PublicOrigin: "https://repopulse.example"})
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
	empty := request("GET", "/api/v1/auth/session", "", "")
	if empty.Code != 200 || strings.TrimSpace(empty.Body.String()) != `{"account":null,"guest":null}` {
		t.Fatalf("empty context: %d %s", empty.Code, empty.Body)
	}
	for _, origin := range []string{"", "null", "https://evil.example", "https://repopulse.example.evil", "https://repopulse.example/path"} {
		w := request("POST", "/api/v1/auth/guest-session", "", origin)
		if w.Code != 403 || store.created != 0 {
			t.Fatalf("origin %q: %d", origin, w.Code)
		}
	}
	created := request("POST", "/api/v1/auth/guest-session", "", "https://repopulse.example")
	if created.Code != 201 || store.created != 1 {
		t.Fatalf("creation: %d %s", created.Code, created.Body)
	}
	cookies := created.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "repopulse_guest" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/v1" || cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("bad cookie attributes: %+v", cookie)
	}
	if strings.Contains(created.Body.String(), cookie.Value) || strings.Contains(created.Body.String(), "guest-principal") {
		t.Fatal("credentials exposed in JSON")
	}
	contextResponse := request("GET", "/api/v1/auth/session", "", "", cookie)
	if contextResponse.Code != 200 || !strings.Contains(contextResponse.Body.String(), `"kind":"guest","login":null`) {
		t.Fatalf("context: %d %s", contextResponse.Code, contextResponse.Body)
	}
	replay := request("POST", "/api/v1/auth/guest-session", "", "https://repopulse.example", cookie)
	if replay.Code != 200 || store.created != 1 || replay.Body.String() != created.Body.String() || replay.Result().Cookies()[0].Value != cookie.Value {
		t.Fatal("guest was duplicated or extended")
	}
	valid := request("POST", "/api/v1/repositories/validate", `{"repository_url":"https://github.com/owner/repo"}`, "https://repopulse.example", cookie)
	if valid.Code != 200 || source.calls != 1 {
		t.Fatalf("validate: %d %s", valid.Code, valid.Body)
	}
	rejected := request("POST", "/api/v1/repositories/validate", `{"repository_url":"https://github.com/owner/repo"}`, "https://repopulse.example", cookie, &http.Cookie{Name: "repopulse_session", Value: "unknown-account"})
	if rejected.Code != 401 || !strings.Contains(rejected.Body.String(), "SESSION_EXPIRED") || source.calls != 1 {
		t.Fatal("invalid account fell back to guest")
	}
	// Revocation/expiry/unknown hashes never reach the repository source.
	for _, mode := range []string{"expired", "revoked", "unknown"} {
		for hash, value := range store.sessions {
			switch mode {
			case "expired":
				value.ExpiresAt = now
			case "revoked":
				value.ExpiresAt = now.Add(time.Hour)
				value.RevokedAt = &now
			case "unknown":
				delete(store.sessions, hash)
				continue
			}
			store.sessions[hash] = value
		}
		w := request("POST", "/api/v1/repositories/validate", `{"repository_url":"https://github.com/owner/repo"}`, "https://repopulse.example", cookie)
		if w.Code != 401 || source.calls != 1 {
			t.Fatalf("%s token accepted", mode)
		}
	}
	// Both mandatory nullable fields remain present.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(contextResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["account"]) != "null" || payload["guest"] == nil {
		t.Fatal("bad nullable fields")
	}
	rpc.Stop()
	unavailable := request("GET", "/api/v1/auth/session", "", "", cookie)
	if unavailable.Code != 503 || !strings.Contains(unavailable.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf("Auth outage: %d %s", unavailable.Code, unavailable.Body)
	}
}
