// Package auth implements the Gateway's binary gRPC client for Auth.
package auth

import (
	"context"
	"time"

	analyzerdomain "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	rpc     authv1.AuthServiceClient
	timeout time.Duration
}

func NewClient(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{rpc: authv1.NewAuthServiceClient(conn), timeout: timeout}
}
func (c *Client) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = metadata.AppendToOutgoingContext(ctx, grpcerr.RequestIDKey, domain.TraceID(ctx))
	return context.WithTimeout(ctx, c.timeout)
}
func (c *Client) GetSessionContext(ctx context.Context, tokens domain.SessionTokens) (domain.SessionContext, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	result, err := c.rpc.GetSessionContext(ctx, toTokens(tokens), grpc.Trailer(&trailer))
	if err != nil {
		return domain.SessionContext{}, toError(err, trailer)
	}
	account, err := fromSession(result.GetAccount())
	if err != nil {
		return domain.SessionContext{}, err
	}
	guest, err := fromSession(result.GetGuest())
	if err != nil {
		return domain.SessionContext{}, err
	}
	if (account != nil && account.Kind != "account") || (guest != nil && guest.Kind != "guest") {
		return domain.SessionContext{}, domain.ErrInternal
	}
	return domain.SessionContext{Account: account, Guest: guest}, nil
}
func (c *Client) CreateGuestSession(ctx context.Context) (domain.IssuedSession, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	result, err := c.rpc.CreateGuestSession(ctx, &commonv1.Empty{}, grpc.Trailer(&trailer))
	if err != nil {
		return domain.IssuedSession{}, toError(err, trailer)
	}
	session, err := fromSession(result.GetSession())
	if err != nil || session == nil || session.Kind != "guest" || result.GetToken() == "" {
		return domain.IssuedSession{}, domain.ErrInternal
	}
	return domain.IssuedSession{Token: result.GetToken(), Session: *session}, nil
}
func (c *Client) Register(ctx context.Context, login, password string) (domain.RegistrationResult, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	result, err := c.rpc.Register(ctx, &authv1.Credentials{Login: login, Password: password}, grpc.Trailer(&trailer))
	if err != nil {
		return domain.RegistrationResult{}, toError(err, trailer)
	}
	if result.GetLogin() == "" || result.GetMessage() == "" {
		return domain.RegistrationResult{}, domain.ErrInternal
	}
	return domain.RegistrationResult{Login: result.GetLogin(), Message: result.GetMessage()}, nil
}

func (c *Client) Login(ctx context.Context, login, password string) (domain.IssuedSession, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	result, err := c.rpc.Login(ctx, &authv1.Credentials{Login: login, Password: password}, grpc.Trailer(&trailer))
	if err != nil {
		return domain.IssuedSession{}, toError(err, trailer)
	}
	session, err := fromSession(result.GetSession())
	if err != nil || session == nil || session.Kind != "account" || result.GetToken() == "" {
		return domain.IssuedSession{}, domain.ErrInternal
	}
	return domain.IssuedSession{Token: result.GetToken(), Session: *session}, nil
}

func (c *Client) Logout(ctx context.Context, accountToken string) error {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	if _, err := c.rpc.Logout(ctx, &authv1.LogoutCommand{AccountToken: accountToken}, grpc.Trailer(&trailer)); err != nil {
		return toError(err, trailer)
	}
	return nil
}

func (c *Client) ResolveContext(ctx context.Context, tokens domain.SessionTokens) (analyzerdomain.VerifiedContext, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	var trailer metadata.MD
	result, err := c.rpc.ResolveContext(ctx, toTokens(tokens), grpc.Trailer(&trailer))
	if err != nil {
		return analyzerdomain.VerifiedContext{}, toError(err, trailer)
	}
	account, err := optionalTime(result.GetAccountExpiresAt())
	if err != nil {
		return analyzerdomain.VerifiedContext{}, err
	}
	guest, err := optionalTime(result.GetGuestExpiresAt())
	if err != nil {
		return analyzerdomain.VerifiedContext{}, err
	}
	return analyzerdomain.VerifiedContext{UserID: result.UserId, GuestSessionID: result.GuestSessionId,
		AccountExpiresAt: account, GuestExpiresAt: guest}, nil
}
func toTokens(tokens domain.SessionTokens) *authv1.SessionTokens {
	return &authv1.SessionTokens{AccountToken: tokens.AccountToken, GuestToken: tokens.GuestToken}
}
func optionalTime(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if value.CheckValid() != nil {
		return nil, domain.ErrInternal
	}
	t := value.AsTime().UTC()
	return &t, nil
}
func fromSession(value *authv1.SessionInfo) (*domain.SessionInfo, error) {
	if value == nil {
		return nil, nil
	}
	expires, err := optionalTime(value.GetExpiresAt())
	if err != nil || expires == nil || (value.GetKind() != "guest" && value.GetKind() != "account") ||
		(value.GetKind() == "guest" && value.Login != nil) || (value.GetKind() == "account" && (value.Login == nil || *value.Login == "")) {
		return nil, domain.ErrInternal
	}
	return &domain.SessionInfo{Kind: value.GetKind(), Login: value.Login, ExpiresAt: *expires}, nil
}

// forwardedCodes are Auth failures whose ApiError is shown to the user as is;
// Auth guarantees their messages are safe (docs/errors.md). Each code is
// accepted only with its gRPC status, so a mismatched trailer cannot relabel
// another failure.
var forwardedCodes = map[string]codes.Code{
	"VALIDATION_FAILED":   codes.InvalidArgument,
	"INVALID_CREDENTIALS": codes.Unauthenticated,
	"LOGIN_TAKEN":         codes.AlreadyExists,
}

func toError(err error, trailer metadata.MD) error {
	if api := trailerError(trailer); api != nil {
		if code, ok := forwardedCodes[api.GetCode()]; ok && code == status.Code(err) && api.GetMessage() != "" {
			fields := make([]domain.FieldError, 0, len(api.GetFieldErrors()))
			for _, f := range api.GetFieldErrors() {
				fields = append(fields, domain.FieldError{Field: f.GetField(), Code: f.GetCode(), Message: f.GetMessage()})
			}
			return &domain.Error{Code: api.GetCode(), Message: api.GetMessage(), FieldErrors: fields}
		}
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return domain.ErrAuthUnavailable
	case codes.Unauthenticated:
		return domain.ErrSessionExpired
	case codes.Unimplemented:
		return &domain.Error{Code: "NOT_IMPLEMENTED", Message: "Операция Auth ещё не реализована"}
	default:
		return domain.ErrInternal
	}
}

// trailerError decodes the ApiError that Auth sends in trailing metadata.
func trailerError(trailer metadata.MD) *commonv1.ApiError {
	values := trailer.Get(grpcerr.TrailerKey)
	if len(values) == 0 {
		return nil
	}
	var api commonv1.ApiError
	if err := proto.Unmarshal([]byte(values[0]), &api); err != nil {
		return nil
	}
	return &api
}
