package report

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	incoming "github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/in/grpc"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type published struct{ report models.Report }

func (p published) GetReport(context.Context, models.VerifiedContext, string) (models.Report, error) {
	return p.report, nil
}

func TestBinaryReportRoundTripPreservesAllContractFields(t *testing.T) {
	for _, name := range []string{"report-zero.json", "report-null.json", "report-partial.json"} {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile("../../../../../../../contracts/examples/" + name)
			if err != nil {
				t.Fatal(err)
			}
			var report models.Report
			if err := json.Unmarshal(body, &report); err != nil {
				t.Fatal(err)
			}
			server := grpc.NewServer(grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
			wire.RegisterReportServiceServer(server, incoming.NewServer(published{report}))
			listener := bufconn.Listen(1 << 20)
			go func() { _ = server.Serve(listener) }()
			defer server.Stop()
			conn, err := grpc.NewClient("passthrough:///report", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			client := NewClient(conn, time.Second)
			decoded, err := client.GetReport(t.Context(), models.VerifiedContext{}, report.ID)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(report)
			after, _ := json.Marshal(decoded)
			var left, right any
			_ = json.Unmarshal(before, &left)
			_ = json.Unmarshal(after, &right)
			if !reflect.DeepEqual(left, right) {
				t.Fatal("binary transport changed report fields, presence, arrays or scalar values")
			}
			server.Stop()
			if _, err := client.GetReport(t.Context(), models.VerifiedContext{}, report.ID); !errors.Is(err, apidomain.ErrReportUnavailable) {
				t.Fatalf("outage: %v", err)
			}
		})
	}
}
