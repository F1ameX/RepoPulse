package grpcerr

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

// trailerStream captures trailers set through grpc.SetTrailer.
type trailerStream struct {
	grpc.ServerTransportStream
	trailer metadata.MD
}

func (s *trailerStream) Method() string { return "/test/Method" }
func (s *trailerStream) SetTrailer(md metadata.MD) error {
	s.trailer = metadata.Join(s.trailer, md)
	return nil
}

func invoke(t *testing.T, ctx context.Context, handlerErr error) (error, *commonv1.ApiError) {
	t.Helper()
	stream := &trailerStream{}
	ctx = grpc.NewContextWithServerTransportStream(ctx, stream)
	_, err := UnaryServerInterceptor()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"},
		func(context.Context, any) (any, error) { return nil, handlerErr })
	values := stream.trailer.Get(TrailerKey)
	if len(values) != 1 {
		t.Fatalf("trailer %s: got %d values", TrailerKey, len(values))
	}
	var api commonv1.ApiError
	if err := proto.Unmarshal([]byte(values[0]), &api); err != nil {
		t.Fatal(err)
	}
	return err, &api
}

func TestInterceptorWritesStatusAndApiError(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDKey, "trace-1"))
	field := &commonv1.FieldError{Field: "login", Code: "FIELD_CODE", Message: "Укажите логин"}
	tests := []struct {
		name    string
		err     error
		code    codes.Code
		apiCode string
		fields  int
	}{
		{"invalid argument", InvalidArgument(field), codes.InvalidArgument, "VALIDATION_FAILED", 1},
		{"invalid credentials", InvalidCredentials(), codes.Unauthenticated, "INVALID_CREDENTIALS", 0},
		{"wrapped", errors.Join(errors.New("context"), NotImplemented()), codes.Unimplemented, "NOT_IMPLEMENTED", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, api := invoke(t, ctx, tt.err)
			if status.Code(err) != tt.code {
				t.Fatalf("status code = %v, want %v", status.Code(err), tt.code)
			}
			if api.GetCode() != tt.apiCode || api.GetTraceId() != "trace-1" || len(api.GetFieldErrors()) != tt.fields {
				t.Fatalf("unexpected ApiError: %v", api)
			}
			if status.Convert(err).Message() != api.GetMessage() {
				t.Fatalf("status message %q differs from ApiError message %q", status.Convert(err).Message(), api.GetMessage())
			}
		})
	}
}

func TestInvalidCredentialsMessage(t *testing.T) {
	if got := InvalidCredentials().APIError().GetMessage(); got != "Неверный логин или пароль" {
		t.Fatalf("message = %q", got)
	}
}

func TestInterceptorHidesUnknownErrors(t *testing.T) {
	err, api := invoke(t, context.Background(), errors.New("dial tcp 10.0.0.1: secret detail"))
	if status.Code(err) != codes.Internal || api.GetCode() != "INTERNAL_ERROR" {
		t.Fatalf("got %v / %v", err, api)
	}
	if status.Convert(err).Message() != "Не удалось выполнить операцию" || api.GetTraceId() != "" {
		t.Fatalf("internal details leaked or trace_id invented: %v / %v", err, api)
	}
}

func TestInterceptorPassesSuccess(t *testing.T) {
	resp, err := UnaryServerInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{},
		func(context.Context, any) (any, error) { return "ok", nil })
	if err != nil || resp != "ok" {
		t.Fatalf("got %v, %v", resp, err)
	}
}
