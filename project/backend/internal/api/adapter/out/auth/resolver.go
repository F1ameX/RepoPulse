// Package auth resolves browser credentials through the Auth gRPC contract.
package auth

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

type ContextClient interface {
	ResolveContext(context.Context, *authv1.SessionTokens, ...grpc.CallOption) (*commonv1.VerifiedContext, error)
}

type Resolver struct {
	client  ContextClient
	timeout time.Duration
}

func NewResolver(client ContextClient, timeout time.Duration) *Resolver {
	return &Resolver{client: client, timeout: timeout}
}

func (r *Resolver) ResolveContext(ctx context.Context, tokens authdomain.SessionTokens) (domain.VerifiedContext, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	var trailer metadata.MD
	verified, err := r.client.ResolveContext(ctx, &authv1.SessionTokens{
		AccountToken: tokens.AccountToken, GuestToken: tokens.GuestToken,
	}, grpc.Trailer(&trailer))
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			for _, raw := range trailer.Get(grpcerr.TrailerKey) {
				var failure commonv1.ApiError
				if proto.Unmarshal([]byte(raw), &failure) == nil && failure.Code == "SESSION_EXPIRED" {
					return domain.VerifiedContext{}, domain.ErrSessionExpired
				}
			}
			return domain.VerifiedContext{}, domain.ErrSessionRequired
		}
		// No provider messages or tokens may enter a public HTTP error.
		return domain.VerifiedContext{}, domain.ErrAuthUnavailable
	}
	if verified == nil || (verified.UserId == nil) != (verified.AccountExpiresAt == nil) ||
		(verified.GuestSessionId == nil) != (verified.GuestExpiresAt == nil) {
		return domain.VerifiedContext{}, domain.ErrAuthUnavailable
	}
	// Never turn a presented account cookie into an implicit guest submission.
	if tokens.AccountToken != nil && verified.UserId == nil {
		return domain.VerifiedContext{}, domain.ErrSessionRequired
	}
	result := domain.VerifiedContext{UserID: verified.UserId, GuestSessionID: verified.GuestSessionId}
	if verified.AccountExpiresAt != nil {
		if verified.AccountExpiresAt.CheckValid() != nil || !domain.ValidID(verified.GetUserId()) {
			return domain.VerifiedContext{}, domain.ErrAuthUnavailable
		}
		at := verified.AccountExpiresAt.AsTime()
		result.AccountExpiresAt = &at
	}
	if verified.GuestExpiresAt != nil {
		if verified.GuestExpiresAt.CheckValid() != nil || !domain.ValidID(verified.GetGuestSessionId()) {
			return domain.VerifiedContext{}, domain.ErrAuthUnavailable
		}
		at := verified.GuestExpiresAt.AsTime()
		result.GuestExpiresAt = &at
	}
	if err := result.Validate(time.Now()); err != nil {
		return domain.VerifiedContext{}, err
	}
	return result, nil
}
