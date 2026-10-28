// Package grpc implements the incoming gRPC adapter of the Auth Service.
package grpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

// Scenarios are the Auth operations called by the gRPC handlers.
type Scenarios interface {
	Register(ctx context.Context, login, password string) (domain.RegistrationResult, error)
	Login(ctx context.Context, login, password string) (domain.IssuedSession, error)
	CreateGuestSession(ctx context.Context) (domain.IssuedSession, error)
	GetSessionContext(ctx context.Context, tokens domain.SessionTokens) (domain.SessionContext, error)
	ResolveContext(ctx context.Context, tokens domain.SessionTokens) (domain.VerifiedContext, error)
	Logout(ctx context.Context, accountToken string) error
}

// Server implements authv1.AuthServiceServer. Errors are returned as
// *grpcerr.Error and need grpcerr.UnaryServerInterceptor on the server.
type Server struct {
	authv1.UnimplementedAuthServiceServer
	scenarios Scenarios
}

func NewServer(scenarios Scenarios) *Server {
	return &Server{scenarios: scenarios}
}

func (s *Server) Register(ctx context.Context, req *authv1.Credentials) (*authv1.RegistrationResult, error) {
	result, err := s.scenarios.Register(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		return nil, toError(err)
	}
	return &authv1.RegistrationResult{Login: result.Login, Message: result.Message}, nil
}

func (s *Server) Login(ctx context.Context, req *authv1.Credentials) (*authv1.IssuedSession, error) {
	issued, err := s.scenarios.Login(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		return nil, toError(err)
	}
	return toIssuedSession(issued), nil
}

func (s *Server) CreateGuestSession(ctx context.Context, _ *commonv1.Empty) (*authv1.IssuedSession, error) {
	issued, err := s.scenarios.CreateGuestSession(ctx)
	if err != nil {
		return nil, toError(err)
	}
	return toIssuedSession(issued), nil
}

func (s *Server) GetSessionContext(ctx context.Context, req *authv1.SessionTokens) (*authv1.SessionContext, error) {
	result, err := s.scenarios.GetSessionContext(ctx, domain.SessionTokens{AccountToken: req.AccountToken, GuestToken: req.GuestToken})
	if err != nil {
		return nil, toError(err)
	}
	return &authv1.SessionContext{Account: toSessionInfo(result.Account), Guest: toSessionInfo(result.Guest)}, nil
}

func toSessionInfo(value *domain.SessionInfo) *authv1.SessionInfo {
	if value == nil {
		return nil
	}
	return &authv1.SessionInfo{Kind: string(value.Kind), Login: value.Login, ExpiresAt: timestamppb.New(value.ExpiresAt)}
}

func (s *Server) ResolveContext(ctx context.Context, req *authv1.SessionTokens) (*commonv1.VerifiedContext, error) {
	verified, err := s.scenarios.ResolveContext(ctx, domain.SessionTokens{
		AccountToken: req.AccountToken,
		GuestToken:   req.GuestToken,
	})
	if err != nil {
		return nil, toError(err)
	}
	return &commonv1.VerifiedContext{
		UserId:           verified.UserID,
		GuestSessionId:   verified.GuestSessionID,
		AccountExpiresAt: toTimestamp(verified.AccountExpiresAt),
		GuestExpiresAt:   toTimestamp(verified.GuestExpiresAt),
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutCommand) (*commonv1.Empty, error) {
	if err := s.scenarios.Logout(ctx, req.GetAccountToken()); err != nil {
		return nil, toError(err)
	}
	return &commonv1.Empty{}, nil
}

func toIssuedSession(issued domain.IssuedSession) *authv1.IssuedSession {
	return &authv1.IssuedSession{
		Token: issued.Token,
		Session: &authv1.SessionInfo{
			Kind:      string(issued.Session.Kind),
			Login:     issued.Session.Login,
			ExpiresAt: timestamppb.New(issued.Session.ExpiresAt),
		},
	}
}

func toTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// toError maps domain errors to the gRPC error format. Unknown errors are
// returned unchanged and become INTERNAL_ERROR in the interceptor.
func toError(err error) error {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		fields := make([]*commonv1.FieldError, 0, len(validation.Fields))
		for _, f := range validation.Fields {
			fields = append(fields, &commonv1.FieldError{Field: f.Field, Code: f.Code, Message: f.Message})
		}
		return grpcerr.InvalidArgument(fields...)
	case errors.Is(err, domain.ErrInvalidCredentials):
		return grpcerr.InvalidCredentials()
	case errors.Is(err, domain.ErrNotImplemented):
		return grpcerr.NotImplemented()
	default:
		return err
	}
}
