package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	authclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/auth"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/application/session"
	authgrpc "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/grpc"
	authpassword "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/password"
	authtoken "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/token"
	authapp "github.com/F1ameX/RepoPulse/project/backend/internal/auth/application/auth"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type accountFixture struct {
	handler http.Handler
	store   *memoryAuthStore
	hasher  *faultableAccountHasher
	logs    bytes.Buffer
}

func newAccountFixture(t *testing.T, fail bool) *accountFixture {
	t.Helper()
	f := &accountFixture{store: newMemoryAuthStore()}
	log := slog.New(slog.NewJSONHandler(&f.logs, nil))
	hasher, err := authpassword.NewArgon2id(authpassword.Params{HashMem: authpassword.MinMemoryKiB, HashTime: 1, HashThreads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	var users authapp.UserRepository = f.store
	if fail {
		users = failingAccountUsers{f.store}
	}
	f.hasher = &faultableAccountHasher{PasswordHasher: hasher}
	svc := authapp.NewService(f.store, users, f.store, authtoken.NewGenerator(), time.Now,
		authdomain.Options{GuestSessionTTL: 24 * time.Hour, AccountSessionTTL: 168 * time.Hour}, f.hasher)
	listener := bufconn.Listen(1 << 20)
	rpc := grpc.NewServer(grpc.ChainUnaryInterceptor(
		func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			return next(logger.With(ctx, log), req)
		}, grpcerr.UnaryServerInterceptor()))
	authv1.RegisterAuthServiceServer(rpc, authgrpc.NewServer(svc))
	go func() { _ = rpc.Serve(listener) }()
	t.Cleanup(rpc.Stop)
	conn, err := grpc.NewClient("passthrough:///auth", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	sessions := session.NewService(authclient.NewClient(conn, 5*time.Second))
	handler := NewHandler(nil, sessions, Options{Sessions: sessions, CookieSecure: true, PublicOrigin: testOrigin})
	f.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(logger.With(r.Context(), log)))
	})
	return f
}

type failingAccountUsers struct{ *memoryAuthStore }

func (f failingAccountUsers) CreateAccount(context.Context, string, string, time.Time) (authdomain.Principal, error) {
	return authdomain.Principal{}, errors.New("database unavailable")
}
func (f failingAccountUsers) FindAccountByLogin(context.Context, string) (authdomain.Principal, error) {
	return authdomain.Principal{}, errors.New("database unavailable")
}

func (f *accountFixture) request(t *testing.T, operation, login, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{"login": login, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/"+operation, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", testOrigin)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func decodedAccountError(t *testing.T, w *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var result errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAccountPasswordNormalizationHTTP(t *testing.T) {
	composed := strings.Repeat("a", 14) + "é"
	decomposed := strings.Repeat("a", 14) + "e\u0301"
	for _, tc := range []struct{ name, registered, entered string }{
		{"NFC registration", composed, decomposed}, {"NFD registration", decomposed, composed},
		{"case and spaces", " Correct horse battery ", " Correct horse battery "},
		{"full long password", strings.Repeat("ж", 128), strings.Repeat("ж", 128)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountFixture(t, false)
			if w := f.request(t, "register", " FlameX ", tc.registered); w.Code != 201 {
				t.Fatalf("register: %d %s", w.Code, w.Body)
			}
			if w := f.request(t, "login", " FLAMEX ", tc.entered); w.Code != 200 {
				t.Fatalf("login: %d %s", w.Code, w.Body)
			}
			for _, wrong := range []string{strings.ToUpper(tc.entered), " " + tc.entered, tc.entered + " ", strings.TrimSpace(tc.entered)} {
				if wrong == tc.entered {
					continue
				}
				// Length violations are covered separately.
				if len([]rune(wrong)) > 128 {
					continue
				}
				before := len(f.store.sessions)
				w := f.request(t, "login", "flamex", wrong)
				if w.Code != 401 || len(w.Result().Cookies()) != 0 || len(f.store.sessions) != before {
					t.Fatalf("changed password: %d %s", w.Code, w.Body)
				}
			}
			if tc.name == "full long password" {
				before := len(f.store.sessions)
				w := f.request(t, "login", "flamex", strings.Repeat("ж", 127)+"з")
				if w.Code != 401 || len(f.store.sessions) != before {
					t.Fatalf("last character ignored: %d %s", w.Code, w.Body)
				}
			}
		})
	}
}

func TestAccountCredentialBoundariesHTTP(t *testing.T) {
	const pass = "correct horse battery"
	for _, tc := range []struct {
		name, login, password string
		valid                 bool
	}{
		{"login 2", "ab", pass, false}, {"login 3", "abc", pass, true},
		{"login 32", strings.Repeat("a", 32), pass, true}, {"login 33", strings.Repeat("a", 33), pass, false},
		{"password 14", "flamex", strings.Repeat("a", 14), false}, {"password 15", "flamex", strings.Repeat("a", 15), true},
		{"password 128", "flamex", strings.Repeat("a", 128), true}, {"password 129", "flamex", strings.Repeat("a", 129), false},
		{"emoji 15", "flamex", strings.Repeat("😀", 15), true}, {"emoji 128", "flamex", strings.Repeat("😀", 128), true},
		{"NFC shrinks below minimum", "flamex", strings.Repeat("a", 13) + "e\u0301", false},
		{"NFC shrinks to maximum", "flamex", strings.Repeat("a", 127) + "e\u0301", true},
		{"spaces only", "flamex", strings.Repeat(" ", 15), false},
		{"login tab", "\tflamex", pass, false}, {"login newline", "flamex\n", pass, false},
		{"login first digit", "1abc", pass, false}, {"login cyrillic", "логин", pass, false},
		{"login dot", "abc.def", pass, false}, {"login hyphen", "abc-def", pass, false}, {"login inner space", "abc def", pass, false},
		{"password tab", "flamex", pass + "\t", false}, {"password newline", "flamex", pass + "\n", false},
		{"password control", "flamex", pass + "\x00", false}, {"password format", "flamex", pass + "\u200b", false},
		{"password other space", "flamex", pass + "\u00a0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountFixture(t, false)
			w := f.request(t, "register", tc.login, tc.password)
			if tc.valid {
				if w.Code != 201 {
					t.Fatalf("register: %d %s", w.Code, w.Body)
				}
				if w = f.request(t, "login", tc.login, tc.password); w.Code != 200 {
					t.Fatalf("login: %d %s", w.Code, w.Body)
				}
			} else {
				if w.Code != 400 {
					t.Fatalf("register: %d %s", w.Code, w.Body)
				}
				if w = f.request(t, "login", tc.login, tc.password); w.Code != 400 {
					t.Fatalf("login: %d %s", w.Code, w.Body)
				}
				if len(f.store.principals) != 0 || len(f.store.sessions) != 0 {
					t.Fatal("invalid credentials changed state")
				}
			}
		})
	}
}

func TestAccountExactFieldErrorsHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, login, password string
		fields                []fieldError
	}{
		{"empty", "   ", "", []fieldError{{Field: "login", Code: "REQUIRED", Message: "Укажите логин"}, {Field: "password", Code: "REQUIRED", Message: "Укажите пароль"}}},
		{"length before characters", "1", "\t", []fieldError{{Field: "login", Code: "INVALID_LENGTH", Message: "Логин должен содержать от 3 до 32 символов"}, {Field: "password", Code: "INVALID_LENGTH", Message: "Пароль должен содержать от 15 до 128 символов"}}},
		{"characters", "1abc", strings.Repeat(" ", 14) + "\t", []fieldError{{Field: "login", Code: "INVALID_CHARACTERS", Message: "Логин: первая латинская буква, далее латинские буквы, цифры или _"}, {Field: "password", Code: "INVALID_CHARACTERS", Message: "Пароль содержит недопустимые символы"}}},
		{"spaces", "flamex", strings.Repeat(" ", 15), []fieldError{{Field: "password", Code: "WHITESPACE_ONLY", Message: "Пароль не может состоять только из пробелов"}}},
		{"short spaces", "flamex", "   ", []fieldError{{Field: "password", Code: "INVALID_LENGTH", Message: "Пароль должен содержать от 15 до 128 символов"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A failing repository proves that format errors take precedence over dependency access.
			f := newAccountFixture(t, true)
			for _, op := range []string{"register", "login"} {
				w := f.request(t, op, tc.login, tc.password)
				got := decodedAccountError(t, w)
				if w.Code != 400 || got.Error.Code != "VALIDATION_FAILED" || !reflect.DeepEqual(got.Error.FieldErrors, tc.fields) {
					t.Fatalf("%s: %d %+v", op, w.Code, got)
				}
			}
			if len(f.store.principals) != 0 || len(f.store.sessions) != 0 {
				t.Fatal("validation changed state")
			}
		})
	}
}

func TestAccountFailuresAndPasswordSecrecyHTTP(t *testing.T) {
	secret := strings.Repeat("a", 14) + "e\u0301"
	normalized := strings.Repeat("a", 14) + "é"
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "storage failure"}[fail], func(t *testing.T) {
			f := newAccountFixture(t, fail)
			assertSecretAbsent := func(w *httptest.ResponseRecorder) {
				t.Helper()
				for _, value := range []string{secret, normalized} {
					if strings.Contains(w.Body.String(), value) {
						t.Fatal("password exposed in response")
					}
				}
			}
			for _, op := range []string{"register", "login"} {
				w := f.request(t, op, "flamex", secret)
				if fail {
					if w.Code != 500 || len(w.Result().Cookies()) != 0 || decodedAccountError(t, w).Error.Code != "INTERNAL_ERROR" {
						t.Fatalf("dependency failure: %d %s", w.Code, w.Body)
					}
				} else {
					want := 201
					if op == "login" {
						want = 200
					}
					if w.Code != want {
						t.Fatalf("success: %d %s", w.Code, w.Body)
					}
				}
				assertSecretAbsent(w)
			}
			if fail && (len(f.store.principals) != 0 || len(f.store.sessions) != 0) {
				t.Fatal("failure changed state")
			}
			w := f.request(t, "login", "flamex", secret+"\t")
			if w.Code != 400 {
				t.Fatalf("validation: %d", w.Code)
			}
			assertSecretAbsent(w)
			if !fail {
				unknown := f.request(t, "login", "unknown", secret)
				wrong := f.request(t, "login", "flamex", secret+"x")
				a, b := decodedAccountError(t, unknown), decodedAccountError(t, wrong)
				a.Error.TraceID = ""
				b.Error.TraceID = ""
				if unknown.Code != 401 || wrong.Code != 401 || len(unknown.Result().Cookies()) != 0 || len(wrong.Result().Cookies()) != 0 || !reflect.DeepEqual(a, b) || a.Error.Message != "Неверный логин или пароль" {
					t.Fatalf("different credential errors: %+v %+v", a, b)
				}
				assertSecretAbsent(unknown)
				assertSecretAbsent(wrong)
			}
			if f.logs.Len() == 0 {
				t.Fatal("no logs captured")
			}
			for _, value := range []string{secret, normalized} {
				if strings.Contains(f.logs.String(), value) {
					t.Fatal("password exposed in logs")
				}
			}
		})
	}
}

type faultableAccountHasher struct {
	authapp.PasswordHasher
	hashErr, verifyErr error
}

func (h *faultableAccountHasher) Hash(password string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return h.PasswordHasher.Hash(password)
}

func (h *faultableAccountHasher) Verify(password, encoded string) (bool, error) {
	if h.verifyErr != nil {
		return false, h.verifyErr
	}
	return h.PasswordHasher.Verify(password, encoded)
}

func TestAccountHasherFailuresHTTP(t *testing.T) {
	for _, operation := range []string{"register", "login"} {
		t.Run(operation, func(t *testing.T) {
			f := newAccountFixture(t, false)
			password := "private password value"
			if operation == "login" {
				if w := f.request(t, "register", "flamex", password); w.Code != 201 {
					t.Fatalf("setup: %d", w.Code)
				}
				f.hasher.verifyErr = errors.New("hash verification unavailable")
			} else {
				f.hasher.hashErr = errors.New("hash generation unavailable")
			}
			beforeAccounts, beforeSessions := len(f.store.principals), len(f.store.sessions)
			w := f.request(t, operation, "flamex", password)
			got := decodedAccountError(t, w)
			if w.Code != 500 || got.Error.Code != "INTERNAL_ERROR" || len(w.Result().Cookies()) != 0 {
				t.Fatalf("hash failure: %d %+v", w.Code, got)
			}
			if len(f.store.principals) != beforeAccounts || len(f.store.sessions) != beforeSessions {
				t.Fatal("hash failure changed state")
			}
			if strings.Contains(w.Body.String(), password) || strings.Contains(f.logs.String(), password) {
				t.Fatal("password exposed on hash failure")
			}
		})
	}
}

func TestAccountNormalizedLoginTakenHTTP(t *testing.T) {
	f := newAccountFixture(t, false)
	if w := f.request(t, "register", "FlameX", "correct horse battery"); w.Code != 201 {
		t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	for _, login := range []string{"flamex", "FLAMEX", " FlameX "} {
		w := f.request(t, "register", login, "another password value")
		got := decodedAccountError(t, w)
		if w.Code != 409 || got.Error.Code != "LOGIN_TAKEN" || got.Error.Message != "Этот логин уже занят" || len(w.Result().Cookies()) != 0 {
			t.Fatalf("duplicate: %d %+v", w.Code, got)
		}
	}
	if len(f.store.principals) != 1 || len(f.store.sessions) != 0 {
		t.Fatal("duplicate registrations changed state")
	}
}
