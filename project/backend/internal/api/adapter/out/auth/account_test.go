package auth

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

// accountServer answers with fixed results or errors; errors go through the
// real grpcerr interceptor, so the ApiError arrives in the trailer as in production.
type accountServer struct {
	authv1.UnimplementedAuthServiceServer
	err        error
	issued     *authv1.IssuedSession
	logoutWith string
}

func (s *accountServer) Register(_ context.Context, req *authv1.Credentials) (*authv1.RegistrationResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &authv1.RegistrationResult{Login: req.GetLogin(), Message: "Аккаунт создан. Войдите в систему"}, nil
}

func (s *accountServer) Login(context.Context, *authv1.Credentials) (*authv1.IssuedSession, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.issued, nil
}

func (s *accountServer) Logout(_ context.Context, req *authv1.LogoutCommand) (*commonv1.Empty, error) {
	s.logoutWith = req.GetAccountToken()
	if s.err != nil {
		return nil, s.err
	}
	return &commonv1.Empty{}, nil
}

func newAccountClient(t *testing.T, server *accountServer) *Client {
	t.Helper()
	return newAccountClientWith(t, server, grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
}

// newRawClient has no grpcerr interceptor: raw statuses reach the client as a
// transport failure or a server without ApiError would produce them.
func newRawClient(t *testing.T, server *accountServer) *Client {
	t.Helper()
	return newAccountClientWith(t, server)
}

func newAccountClientWith(t *testing.T, server *accountServer, options ...grpc.ServerOption) *Client {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	rpc := grpc.NewServer(options...)
	authv1.RegisterAuthServiceServer(rpc, server)
	go func() { _ = rpc.Serve(listener) }()
	t.Cleanup(rpc.Stop)
	conn, err := grpc.NewClient("passthrough:///auth",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewClient(conn, time.Second)
}

func TestRegisterForwardsFieldErrorsAndLoginTaken(t *testing.T) {
	validation := grpcerr.InvalidArgument(
		&commonv1.FieldError{Field: "login", Code: "REQUIRED", Message: "Укажите логин"},
		&commonv1.FieldError{Field: "password", Code: "INVALID_LENGTH", Message: "Пароль должен содержать от 15 до 128 символов"},
	)
	_, err := newAccountClient(t, &accountServer{err: validation}).Register(context.Background(), "", "short")
	failure, ok := errors.AsType[*domain.Error](err)
	if !ok || failure.Code != "VALIDATION_FAILED" || len(failure.FieldErrors) != 2 ||
		failure.FieldErrors[0] != (domain.FieldError{Field: "login", Code: "REQUIRED", Message: "Укажите логин"}) {
		t.Fatalf("err = %+v; want VALIDATION_FAILED with both field errors", err)
	}

	_, err = newAccountClient(t, &accountServer{err: grpcerr.LoginTaken()}).Register(context.Background(), "flamex", "correct horse battery")
	if failure, ok := errors.AsType[*domain.Error](err); !ok || failure.Code != "LOGIN_TAKEN" || failure.Message != "Этот логин уже занят" {
		t.Fatalf("err = %+v; want LOGIN_TAKEN", err)
	}

	result, err := newAccountClient(t, &accountServer{}).Register(context.Background(), "flamex", "correct horse battery")
	if err != nil || result.Login != "flamex" || result.Message == "" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

// Both are UNAUTHENTICATED in gRPC; the ApiError code must keep them apart.
func TestInvalidCredentialsIsNotSessionExpired(t *testing.T) {
	_, err := newAccountClient(t, &accountServer{err: grpcerr.InvalidCredentials()}).Login(context.Background(), "flamex", "wrong password value")
	if failure, ok := errors.AsType[*domain.Error](err); !ok || failure.Code != "INVALID_CREDENTIALS" || failure.Message != "Неверный логин или пароль" {
		t.Fatalf("err = %+v; want INVALID_CREDENTIALS", err)
	}

	// Without an ApiError the previous mapping stays: UNAUTHENTICATED means an invalid session.
	_, err = newRawClient(t, &accountServer{err: status.Error(codes.Unauthenticated, "raw")}).Login(context.Background(), "flamex", "x")
	if !errors.Is(err, domain.ErrSessionExpired) {
		t.Fatalf("err = %v; want SESSION_EXPIRED", err)
	}
}

func TestMismatchedTrailerIsNotForwarded(t *testing.T) {
	// A forwarded code with an unexpected status must not relabel the failure.
	mismatched := grpcerr.New(codes.Internal, "INVALID_CREDENTIALS", "Неверный логин или пароль")
	_, err := newAccountClient(t, &accountServer{err: mismatched}).Login(context.Background(), "flamex", "x")
	if !errors.Is(err, domain.ErrInternal) {
		t.Fatalf("err = %v; want INTERNAL_ERROR", err)
	}
	// A failure that is not in the allow list keeps its safe Gateway message.
	_, err = newAccountClient(t, &accountServer{err: errors.New("dial tcp 10.0.0.1: secret")}).Login(context.Background(), "flamex", "x")
	if !errors.Is(err, domain.ErrInternal) {
		t.Fatalf("err = %v; want INTERNAL_ERROR", err)
	}
}

func TestLoginResponseIsValidated(t *testing.T) {
	login := "flamex"
	expires := timestamppb.New(time.Now().Add(time.Hour))
	issued, err := newAccountClient(t, &accountServer{issued: &authv1.IssuedSession{
		Token: "token", Session: &authv1.SessionInfo{Kind: "account", Login: &login, ExpiresAt: expires},
	}}).Login(context.Background(), "flamex", "correct horse battery")
	if err != nil || issued.Token != "token" || issued.Session.Kind != "account" || *issued.Session.Login != "flamex" {
		t.Fatalf("issued = %+v, %v", issued, err)
	}

	for name, bad := range map[string]*authv1.IssuedSession{
		"guest session":   {Token: "token", Session: &authv1.SessionInfo{Kind: "guest", ExpiresAt: expires}},
		"missing token":   {Session: &authv1.SessionInfo{Kind: "account", Login: &login, ExpiresAt: expires}},
		"missing login":   {Token: "token", Session: &authv1.SessionInfo{Kind: "account", ExpiresAt: expires}},
		"missing session": {Token: "token"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newAccountClient(t, &accountServer{issued: bad}).Login(context.Background(), "flamex", "x")
			if !errors.Is(err, domain.ErrInternal) {
				t.Fatalf("err = %v; malformed response must fail closed", err)
			}
		})
	}
}

func TestLogoutSendsTokenAndMapsOutage(t *testing.T) {
	server := &accountServer{}
	if err := newAccountClient(t, server).Logout(context.Background(), "account-token"); err != nil || server.logoutWith != "account-token" {
		t.Fatalf("Logout = %v, token = %q", err, server.logoutWith)
	}
	err := newRawClient(t, &accountServer{err: status.Error(codes.Unavailable, "down")}).Logout(context.Background(), "t")
	if !errors.Is(err, domain.ErrAuthUnavailable) {
		t.Fatalf("err = %v; want AUTH_UNAVAILABLE", err)
	}
}
