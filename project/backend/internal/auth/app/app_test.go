package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	authv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/auth/v1"
)

func TestServerHealthStubAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(logger.With(context.Background(), log))
	defer cancel()
	// The pool is only passed to repositories; the calls below do not reach the database.
	server, healthServer := newGRPCServer(ctx, nil)
	done := make(chan error, 1)
	go func() { done <- serve(ctx, listener, server, healthServer, time.Second) }()

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	health := healthpb.NewHealthClient(conn)
	for _, service := range []string{"", "repopulse.auth.v1.AuthService"} {
		resp, err := health.Check(callCtx, &healthpb.HealthCheckRequest{Service: service}, grpc.WaitForReady(true))
		if err != nil {
			t.Fatalf("health %q: %v", service, err)
		}
		if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			t.Fatalf("health %q = %v", service, resp.GetStatus())
		}
	}

	_, err = authv1.NewAuthServiceClient(conn).Register(callCtx, &authv1.Credentials{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("Register = %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}
