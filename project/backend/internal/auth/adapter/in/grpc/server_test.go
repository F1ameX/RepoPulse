package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

func newClient(t *testing.T, scenarios Scenarios) authv1.AuthServiceClient {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
	authv1.RegisterAuthServiceServer(server, NewServer(scenarios))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return authv1.NewAuthServiceClient(conn)
}

func apiError(t *testing.T, trailer metadata.MD) *commonv1.ApiError {
	t.Helper()
	values := trailer.Get(grpcerr.TrailerKey)
	if len(values) != 1 {
		t.Fatalf("missing %s trailer: %v", grpcerr.TrailerKey, trailer)
	}
	var api commonv1.ApiError
	if err := proto.Unmarshal([]byte(values[0]), &api); err != nil {
		t.Fatal(err)
	}
	return &api
}

// Every Auth method is implemented; the mapping of a scaffold error and the
// trace propagation are still part of the shared error format.
func TestNotImplementedKeepsTraceID(t *testing.T) {
	client := newClient(t, fakeScenarios{loginErr: domain.ErrNotImplemented})
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpcerr.RequestIDKey, "req-1")
	var trailer metadata.MD
	_, err := client.Login(ctx, &authv1.Credentials{}, grpc.Trailer(&trailer))
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("status = %v", err)
	}
	api := apiError(t, trailer)
	if api.GetCode() != "NOT_IMPLEMENTED" || api.GetTraceId() != "req-1" {
		t.Fatalf("ApiError = %v", api)
	}
}

type fakeScenarios struct {
	Scenarios
	loginErr error
	issued   domain.IssuedSession
	verified domain.VerifiedContext
}

func (f fakeScenarios) Login(context.Context, string, string) (domain.IssuedSession, error) {
	return f.issued, f.loginErr
}

func (f fakeScenarios) ResolveContext(context.Context, domain.SessionTokens) (domain.VerifiedContext, error) {
	return f.verified, nil
}

func TestDomainErrorsMapToErrorFormat(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    codes.Code
		apiCode string
		fields  []*commonv1.FieldError
	}{
		{
			name: "malformed request",
			err: &domain.ValidationError{Fields: []domain.FieldError{
				{Field: "login", Code: "FIELD_CODE", Message: "Укажите логин"},
				{Field: "password", Code: "FIELD_CODE", Message: "Укажите пароль"},
			}},
			code: codes.InvalidArgument, apiCode: "VALIDATION_FAILED",
			fields: []*commonv1.FieldError{
				{Field: "login", Code: "FIELD_CODE", Message: "Укажите логин"},
				{Field: "password", Code: "FIELD_CODE", Message: "Укажите пароль"},
			},
		},
		{name: "unavailable user", err: domain.ErrInvalidCredentials, code: codes.Unauthenticated, apiCode: "INVALID_CREDENTIALS"},
		{name: "login taken", err: domain.ErrLoginTaken, code: codes.AlreadyExists, apiCode: "LOGIN_TAKEN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newClient(t, fakeScenarios{loginErr: tt.err})
			var trailer metadata.MD
			_, err := client.Login(context.Background(), &authv1.Credentials{}, grpc.Trailer(&trailer))
			if status.Code(err) != tt.code {
				t.Fatalf("status = %v", err)
			}
			api := apiError(t, trailer)
			if api.GetCode() != tt.apiCode || len(api.GetFieldErrors()) != len(tt.fields) {
				t.Fatalf("ApiError = %v", api)
			}
			for i, want := range tt.fields {
				if !proto.Equal(api.GetFieldErrors()[i], want) {
					t.Fatalf("field_errors[%d] = %v, want %v", i, api.GetFieldErrors()[i], want)
				}
			}
		})
	}
}

func TestSuccessResponsesKeepPresence(t *testing.T) {
	login := "flamex"
	expires := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	userID := "4f9d4c1e-0000-4000-8000-000000000001"
	client := newClient(t, fakeScenarios{
		issued: domain.IssuedSession{Token: "t", Session: domain.SessionInfo{
			Kind: domain.KindAccount, Login: &login, ExpiresAt: expires,
		}},
		verified: domain.VerifiedContext{UserID: &userID, AccountExpiresAt: &expires},
	})

	issued, err := client.Login(context.Background(), &authv1.Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	session := issued.GetSession()
	if session.GetKind() != "account" || session.Login == nil || *session.Login != login ||
		!session.GetExpiresAt().AsTime().Equal(expires) {
		t.Fatalf("session = %v", session)
	}

	verified, err := client.ResolveContext(context.Background(), &authv1.SessionTokens{})
	if err != nil {
		t.Fatal(err)
	}
	if verified.GetUserId() != userID || verified.GuestSessionId != nil ||
		verified.GetAccountExpiresAt() == nil || verified.GetGuestExpiresAt() != nil {
		t.Fatalf("verified context = %v", verified)
	}
}
