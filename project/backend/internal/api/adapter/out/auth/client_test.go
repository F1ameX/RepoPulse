package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeConn struct {
	invoke func(context.Context, string, any, any) error
}

func (c fakeConn) Invoke(ctx context.Context, method string, args, reply any, _ ...grpc.CallOption) error {
	return c.invoke(ctx, method, args, reply)
}
func (c fakeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unexpected stream")
}

func TestAuthTimeoutAndErrorsAreBoundedAndSafe(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.Unimplemented, codes.Unauthenticated} {
		client := NewClient(fakeConn{invoke: func(context.Context, string, any, any) error {
			return status.Error(code, "private token and internal address")
		}}, time.Second)
		_, err := client.GetSessionContext(context.Background(), domain.SessionTokens{})
		var failure *domain.Error
		if !errors.As(err, &failure) || failure.Message == "private token and internal address" {
			t.Fatalf("unsafe error: %v", err)
		}
		if (code == codes.Unavailable || code == codes.DeadlineExceeded) && failure.Code != "AUTH_UNAVAILABLE" {
			t.Fatalf("bad code: %v", err)
		}
	}
	client := NewClient(fakeConn{invoke: func(ctx context.Context, _ string, _, _ any) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing RPC deadline")
		}
		<-ctx.Done()
		return status.Error(codes.DeadlineExceeded, "timeout")
	}}, 10*time.Millisecond)
	start := time.Now()
	if _, err := client.GetSessionContext(context.Background(), domain.SessionTokens{}); !errors.Is(err, domain.ErrAuthUnavailable) {
		t.Fatalf("timeout: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("RPC timeout was not applied")
	}
}

func TestMalformedSessionResponseFailsClosed(t *testing.T) {
	login := "account-login"
	for _, info := range []*authv1.SessionInfo{
		{Kind: "guest"},
		{Kind: "guest", ExpiresAt: &timestamppb.Timestamp{Seconds: 1 << 62}},
		{Kind: "account", ExpiresAt: timestamppb.Now()},
		{Kind: "unexpected", ExpiresAt: timestamppb.Now()},
		{Kind: "account", Login: &login, ExpiresAt: timestamppb.Now()},
	} {
		client := NewClient(fakeConn{invoke: func(_ context.Context, _ string, _ any, reply any) error {
			reply.(*authv1.SessionContext).Guest = info
			return nil
		}}, time.Second)
		if _, err := client.GetSessionContext(context.Background(), domain.SessionTokens{}); !errors.Is(err, domain.ErrInternal) {
			t.Fatalf("malformed session accepted: %+v, %v", info, err)
		}
	}
}
