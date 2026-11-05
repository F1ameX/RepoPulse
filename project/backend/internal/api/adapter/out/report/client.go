package report

import (
	"context"
	"math"
	"strings"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	common "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	rpc     wire.ReportServiceClient
	timeout time.Duration
}

func NewClient(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{rpc: wire.NewReportServiceClient(conn), timeout: timeout}
}

func (c *Client) GetReport(ctx context.Context, verified models.VerifiedContext, id string) (models.Report, error) {
	id = strings.ToLower(id)
	ctx = metadata.AppendToOutgoingContext(ctx, grpcerr.RequestIDKey, apidomain.TraceID(ctx))
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	result, err := c.rpc.GetReport(ctx, &wire.GetReportQuery{ReportId: id, Context: &common.VerifiedContext{
		UserId: verified.UserID, GuestSessionId: verified.GuestSessionID, AccountExpiresAt: toTimestamp(verified.AccountExpiresAt), GuestExpiresAt: toTimestamp(verified.GuestExpiresAt)}})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return models.Report{}, models.ErrNotFound
		case codes.Unauthenticated:
			return models.Report{}, models.ErrSessionExpired
		case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
			return models.Report{}, apidomain.ErrReportUnavailable
		default:
			return models.Report{}, apidomain.ErrInternal
		}
	}
	if result == nil || result.ReportId != id || result.AssessmentAt == nil || result.CompletedAt == nil ||
		result.Window == nil || result.Window.From == nil || result.Window.Until == nil || result.Repository == nil || result.Versions == nil ||
		result.AssessmentAt.CheckValid() != nil || result.CompletedAt.CheckValid() != nil || result.Window.From.CheckValid() != nil || result.Window.Until.CheckValid() != nil {
		return models.Report{}, apidomain.ErrInternal
	}
	if !validMessage(result.ProtoReflect()) {
		return models.Report{}, apidomain.ErrInternal
	}
	report := fromReport(result)
	inputs := []models.MetricInput{}
	for _, category := range report.Categories {
		for _, metric := range category.Metrics {
			inputs = append(inputs, metric.Input)
		}
	}
	data := models.SnapshotData{Refs: []models.GitRef{}, Sources: report.Sources, Inputs: inputs, Dependencies: report.Dependencies, Vulnerabilities: []models.Vulnerability{}}
	if data.Validate() != nil || report.Repository.Validate() != nil || report.Versions.Validate() != nil ||
		!report.Window.Until.Equal(report.AssessmentAt) || !report.Window.From.Equal(report.AssessmentAt.Add(-140*24*time.Hour)) {
		return models.Report{}, apidomain.ErrInternal
	}
	return report, nil
}

// Reject malformed protobuf timestamps/scalars before mapping to HTTP values.
func validMessage(message protoreflect.Message) bool {
	switch value := message.Interface().(type) {
	case *wire.Metric:
		if value.Input == nil {
			return false
		}
	case *common.MetricInput:
		if value.RawValue == nil {
			return false
		}
	case *common.Window:
		if value.From == nil || value.Until == nil {
			return false
		}
	case *common.Vulnerability:
		if value.ObtainedAt == nil {
			return false
		}
	case *timestamppb.Timestamp:
		return value.CheckValid() == nil
	case *structpb.Value:
		switch scalar := value.Kind.(type) {
		case *structpb.Value_NullValue, *structpb.Value_BoolValue, *structpb.Value_StringValue:
			return true
		case *structpb.Value_NumberValue:
			return !math.IsNaN(scalar.NumberValue) && !math.IsInf(scalar.NumberValue, 0)
		default:
			return false
		}
	}
	valid := true
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() == protoreflect.DoubleKind || field.Kind() == protoreflect.FloatKind {
			number := value.Float()
			if math.IsNaN(number) || math.IsInf(number, 0) {
				valid = false
				return false
			}
		}
		if field.Message() == nil {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if !validMessage(list.Get(i).Message()) {
					valid = false
					break
				}
			}
		} else {
			valid = validMessage(value.Message())
		}
		return valid
	})
	return valid
}

func toTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
