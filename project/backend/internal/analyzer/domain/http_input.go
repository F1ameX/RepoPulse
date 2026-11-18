package domain

import (
	"context"
	"errors"
	"io"
	"sync"
)

const (
	HTTPPageLimit     = 10
	HTTPResponseLimit = 5 << 20
	HTTPInputLimit    = 50 << 20
)

var (
	ErrHTTPResponseLimit = errors.New("HTTP response exceeds byte budget")
	ErrHTTPInputLimit    = errors.New("analysis HTTP input budget exhausted")
)

type httpInputKey struct{}

type httpInputBudget struct {
	mu        sync.Mutex
	remaining int64
}

// WithHTTPInputBudget keeps one budget across external HTTP sources of a job.
// Git acquisition has a separate profile and does not consume this budget.
func WithHTTPInputBudget(ctx context.Context) context.Context {
	if _, ok := ctx.Value(httpInputKey{}).(*httpInputBudget); ok {
		return ctx
	}
	return context.WithValue(ctx, httpInputKey{}, &httpInputBudget{remaining: HTTPInputLimit})
}

func HTTPInputExhausted(ctx context.Context) bool {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return false
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.remaining == 0
}

// ReadHTTPInput charges bytes even on a truncated or oversized response.
func ReadHTTPInput(ctx context.Context, reader io.Reader) ([]byte, error) {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	limit := int64(HTTPResponseLimit + 1)
	if budget != nil {
		budget.mu.Lock()
		defer budget.mu.Unlock()
		limit = min(limit, budget.remaining)
	}
	if limit == 0 {
		return nil, ErrHTTPInputLimit
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit))
	if budget != nil {
		budget.remaining -= int64(len(body))
		if budget.remaining == 0 {
			return nil, ErrHTTPInputLimit
		}
	}
	if len(body) > HTTPResponseLimit {
		return nil, ErrHTTPResponseLimit
	}
	return body, err
}
