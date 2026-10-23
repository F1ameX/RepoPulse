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
	result, err := c.rpc.GetSessionContext(ctx, toTokens(tokens))
	if err != nil {
		return domain.SessionContext{}, toError(err)
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
	result, err := c.rpc.CreateGuestSession(ctx, &commonv1.Empty{})
	if err != nil {
		return domain.IssuedSession{}, toError(err)
	}
	session, err := fromSession(result.GetSession())
	if err != nil || session == nil || session.Kind != "guest" || result.GetToken() == "" {
		return domain.IssuedSession{}, domain.ErrInternal
	}
	return domain.IssuedSession{Token: result.GetToken(), Session: *session}, nil
}
func (c *Client) ResolveContext(ctx context.Context, tokens domain.SessionTokens) (analyzerdomain.VerifiedContext, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	result, err := c.rpc.ResolveContext(ctx, toTokens(tokens))
	if err != nil {
		return analyzerdomain.VerifiedContext{}, toError(err)
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
func toError(err error) error {
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
