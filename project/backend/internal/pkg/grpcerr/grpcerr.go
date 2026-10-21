// Package grpcerr implements the RepoPulse gRPC error format from docs/errors.md:
// a standard gRPC status plus common.v1.ApiError in binary trailing metadata.
package grpcerr

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
	commonv1 "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/common/v1"
)

const (
	// TrailerKey carries the serialized ApiError.
	TrailerKey = "repopulse-error-bin"
	// RequestIDKey is the incoming metadata copied into ApiError.trace_id.
	RequestIDKey = "x-request-id"
)

// Error is a handler error that the interceptor converts into status + ApiError.
type Error struct {
	code codes.Code
	api  *commonv1.ApiError
}

func (e *Error) Error() string {
	return e.code.String() + ": " + e.api.GetCode()
}

// Code returns the gRPC status code.
func (e *Error) Code() codes.Code { return e.code }

// APIError returns a copy of the ApiError without trace_id.
func (e *Error) APIError() *commonv1.ApiError {
	return proto.CloneOf(e.api)
}

// New builds an error. message must be safe to show to users.
func New(code codes.Code, apiCode, message string, fieldErrors ...*commonv1.FieldError) *Error {
	return &Error{code: code, api: &commonv1.ApiError{
		Code:        apiCode,
		Message:     message,
		FieldErrors: fieldErrors,
	}}
}

// InvalidArgument reports a malformed request with the first error of each field.
func InvalidArgument(fieldErrors ...*commonv1.FieldError) *Error {
	return New(codes.InvalidArgument, "VALIDATION_FAILED", "Исправьте ошибки в полях запроса", fieldErrors...)
}

// InvalidCredentials reports an unavailable user. Unknown login and wrong
// password share this error so the existence of a login is not disclosed.
func InvalidCredentials() *Error {
	return New(codes.Unauthenticated, "INVALID_CREDENTIALS", "Неверный логин или пароль")
}

// NotImplemented marks a scaffold method; it does not confirm the feature exists.
func NotImplemented() *Error {
	return New(codes.Unimplemented, "NOT_IMPLEMENTED", "Method is not implemented yet")
}

// Internal hides details of an unexpected failure.
func Internal() *Error {
	return New(codes.Internal, "INTERNAL_ERROR", "Не удалось выполнить операцию")
}

// UnaryServerInterceptor converts handler errors into the RepoPulse format.
// Errors other than *Error are logged and returned as INTERNAL_ERROR.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		var e *Error
		if !errors.As(err, &e) {
			logger.From(ctx).ErrorContext(ctx, "gRPC handler failed", "method", info.FullMethod, "error", err)
			e = Internal()
		}
		api := e.APIError()
		api.TraceId = traceID(ctx)
		if body, err := proto.Marshal(api); err == nil {
			if err := grpc.SetTrailer(ctx, metadata.Pairs(TrailerKey, string(body))); err != nil {
				logger.From(ctx).WarnContext(ctx, "Set gRPC error trailer", "method", info.FullMethod, "error", err)
			}
		}
		return nil, status.Error(e.code, api.GetMessage())
	}
}

func traceID(ctx context.Context) string {
	if values := metadata.ValueFromIncomingContext(ctx, RequestIDKey); len(values) > 0 {
		return values[0]
	}
	return ""
}
