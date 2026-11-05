package repository_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/report/app"
	common "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestReportProcessStartsAgainstMigratedStorageAndStops(t *testing.T) {
	pool := testDatabase(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	t.Setenv("REPORT_DATABASE_URL", pool.Config().ConnString())
	t.Setenv("REPORT_GRPC_ADDR", address)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Report process: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Report process did not stop")
		}
	}()
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	health := grpc_health_v1.NewHealthClient(conn)
	deadline := time.Now().Add(5 * time.Second)
	for {
		attempt, stop := context.WithTimeout(t.Context(), 200*time.Millisecond)
		response, err := health.Check(attempt, &grpc_health_v1.HealthCheckRequest{Service: wire.ReportService_ServiceDesc.ServiceName})
		stop()
		if err == nil && response.Status == grpc_health_v1.HealthCheckResponse_SERVING {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Report readiness: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	guest := "00000000-0000-4000-8000-000000000001"
	_, err = wire.NewReportServiceClient(conn).GetReport(t.Context(), &wire.GetReportQuery{
		ReportId: "00000000-0000-4000-8000-000000000002", Context: &common.VerifiedContext{GuestSessionId: &guest, GuestExpiresAt: timestamppb.New(time.Now().Add(time.Hour))}})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("empty report storage: %v", err)
	}
}
