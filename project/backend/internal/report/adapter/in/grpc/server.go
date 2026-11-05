package grpc

import (
	"context"
	"errors"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	"github.com/F1ameX/RepoPulse/project/backend/internal/report/domain"
	wire "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Reports interface {
	GetReport(context.Context, domain.VerifiedContext, string) (domain.Report, error)
}

type Server struct {
	wire.UnimplementedReportServiceServer
	reports Reports
}

func NewServer(reports Reports) *Server { return &Server{reports: reports} }

func (s *Server) ListReports(context.Context, *wire.ListReportsQuery) (*wire.HistoryPage, error) {
	return nil, grpcerr.NotImplemented()
}

func (s *Server) FindReusableReport(context.Context, *wire.ReusableReportQuery) (*wire.ReusableReportResult, error) {
	return nil, grpcerr.NotImplemented()
}

func (s *Server) GetReport(ctx context.Context, request *wire.GetReportQuery) (*wire.Report, error) {
	verified := domain.VerifiedContext{}
	if value := request.GetContext(); value != nil {
		if (value.AccountExpiresAt != nil && value.AccountExpiresAt.CheckValid() != nil) ||
			(value.GuestExpiresAt != nil && value.GuestExpiresAt.CheckValid() != nil) {
			return nil, grpcerr.New(codes.Unauthenticated, "SESSION_REQUIRED", "Нужна действующая сессия")
		}
		verified = domain.VerifiedContext{UserID: value.UserId, GuestSessionID: value.GuestSessionId,
			AccountExpiresAt: fromTimestamp(value.AccountExpiresAt), GuestExpiresAt: fromTimestamp(value.GuestExpiresAt)}
	}
	report, err := s.reports.GetReport(ctx, verified, request.GetReportId())
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return nil, grpcerr.New(codes.NotFound, "REPORT_NOT_FOUND", "Отчёт недоступен")
	case errors.Is(err, models.ErrSessionRequired), errors.Is(err, models.ErrSessionExpired):
		return nil, grpcerr.New(codes.Unauthenticated, "SESSION_EXPIRED", "Срок действия сессии истёк")
	case err != nil:
		return nil, err
	}
	return toReport(report), nil
}

func fromTimestamp(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	at := value.AsTime().UTC()
	return &at
}
