package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

type contextClientFunc func(context.Context, *authv1.SessionTokens) (*commonv1.VerifiedContext, error)

func (f contextClientFunc) ResolveContext(ctx context.Context, tokens *authv1.SessionTokens, _ ...grpc.CallOption) (*commonv1.VerifiedContext, error) {
	return f(ctx, tokens)
}

func TestResolveContextDistinguishesAuthOutageAndRejectsAccountFallback(t *testing.T) {
	id, token := domain.NewID(), "opaque"
	guest := &commonv1.VerifiedContext{GuestSessionId: &id, GuestExpiresAt: timestamppb.New(time.Now().Add(time.Hour))}
	for _, tc := range []struct {
		name      string
		tokens    authdomain.SessionTokens
		response  *commonv1.VerifiedContext
		err, want error
	}{
		{"guest", authdomain.SessionTokens{GuestToken: &token}, guest, nil, nil},
		{"outage", authdomain.SessionTokens{GuestToken: &token}, nil, status.Error(codes.Unavailable, "private detail"), domain.ErrAuthUnavailable},
		{"timeout", authdomain.SessionTokens{GuestToken: &token}, nil, status.Error(codes.DeadlineExceeded, "private detail"), domain.ErrAuthUnavailable},
		{"unknown session", authdomain.SessionTokens{GuestToken: &token}, &commonv1.VerifiedContext{}, nil, domain.ErrSessionRequired},
		{"account cannot become guest", authdomain.SessionTokens{AccountToken: &token, GuestToken: &token}, guest, nil, domain.ErrSessionRequired},
		{"incomplete identity", authdomain.SessionTokens{GuestToken: &token}, &commonv1.VerifiedContext{GuestSessionId: &id}, nil, domain.ErrAuthUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := contextClientFunc(func(_ context.Context, tokens *authv1.SessionTokens) (*commonv1.VerifiedContext, error) {
				if (tokens.AccountToken == nil) != (tc.tokens.AccountToken == nil) || (tokens.GuestToken == nil) != (tc.tokens.GuestToken == nil) {
					t.Fatal("optional token presence changed")
				}
				return tc.response, tc.err
			})
			got, err := NewResolver(client, time.Second).ResolveContext(t.Context(), tc.tokens)
			if !errors.Is(err, tc.want) || (tc.want == nil && got.GuestSessionID == nil) {
				t.Fatalf("context=%+v error=%#v want=%#v", got, err, tc.want)
			}
		})
	}
}
