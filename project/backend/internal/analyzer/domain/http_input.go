package domain

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	JobTimeLimit      = 320 * time.Second
	CoreTimeLimit     = 300 * time.Second
	ExplainTimeLimit  = 20 * time.Second
	HTTPAttemptTime   = 15 * time.Second
	HTTPMaxTries      = 2
	GitHubHTTPLimit   = 20
	OSVHTTPLimit      = 40
	LLMHTTPLimit      = 1
	JobHTTPLimit      = 61
	GitHubObjectLimit = 1000
	HTTPPageLimit     = 10
	HTTPResponseLimit = 5 << 20
	HTTPInputLimit    = 50 << 20
)

var (
	ErrHTTPResponseLimit = errors.New("HTTP response exceeds byte budget")
	ErrHTTPInputLimit    = errors.New("analysis HTTP input budget exhausted")
	ErrHTTPAttemptLimit  = errors.New("HTTP attempt budget exhausted")
)

type HTTPSource uint8

const (
	HTTPGitHub HTTPSource = iota
	HTTPOSV
	HTTPLLM
)

var attemptLimits = [...]int{GitHubHTTPLimit, OSVHTTPLimit, LLMHTTPLimit}

type httpInputKey struct{}

type httpInputBudget struct {
	mu        sync.Mutex // attempts and GitHub object identities
	readMu    sync.Mutex // serialize charging response bytes, including failed reads
	remaining int64
	total     int
	attempts  [len(attemptLimits)]int
	objects   map[string]struct{}
}

// WithHTTPInputBudget keeps one budget across the job's HTTP sources. Git
// requests can charge attempts here while streaming pack data under a separate byte cap.
func WithHTTPInputBudget(ctx context.Context) context.Context {
	if _, ok := ctx.Value(httpInputKey{}).(*httpInputBudget); ok {
		return ctx
	}
	return context.WithValue(ctx, httpInputKey{}, &httpInputBudget{remaining: HTTPInputLimit, objects: make(map[string]struct{})})
}

// HTTPAttemptAvailable checks admission without charging, e.g. before a backoff.
func HTTPAttemptAvailable(ctx context.Context, source HTTPSource) error {
	return httpAttempt(ctx, source, false)
}

// HTTPAttemptsRemaining permits reserving part of the budget for another scope.
func HTTPAttemptsRemaining(ctx context.Context, source HTTPSource) int {
	if int(source) >= len(attemptLimits) {
		return 0
	}
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return attemptLimits[source]
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return max(0, min(JobHTTPLimit-budget.total, attemptLimits[source]-budget.attempts[source]))
}

// TakeHTTPAttempt must be called immediately before making an outbound request.
func TakeHTTPAttempt(ctx context.Context, source HTTPSource) error {
	return httpAttempt(ctx, source, true)
}

func httpAttempt(ctx context.Context, source HTTPSource, charge bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if int(source) >= len(attemptLimits) {
		return ErrHTTPAttemptLimit
	}
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return nil
	}
	if HTTPInputExhausted(ctx) {
		return ErrHTTPInputLimit
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if budget.total >= JobHTTPLimit || budget.attempts[source] >= attemptLimits[source] {
		return ErrHTTPAttemptLimit
	}
	if charge {
		budget.total++
		budget.attempts[source]++
	}
	return nil
}

func GitHubObjectsExhausted(ctx context.Context) bool {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return false
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return len(budget.objects) >= GitHubObjectLimit
}

// RecordGitHubObject keeps one slot per identity, even if that identity is
// later excluded from the sample as invalid or conflicting.
func RecordGitHubObject(ctx context.Context, identity string) bool {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return true
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if _, seen := budget.objects[identity]; seen {
		return true
	}
	if len(budget.objects) >= GitHubObjectLimit {
		return false
	}
	budget.objects[identity] = struct{}{}
	return true
}

// HTTPRetryFits avoids starting a wait which cannot leave time for another try.
func HTTPRetryFits(ctx context.Context, delay time.Duration) bool {
	deadline, bounded := ctx.Deadline()
	return ctx.Err() == nil && (!bounded || time.Until(deadline) > delay)
}

func HTTPInputExhausted(ctx context.Context) bool {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget == nil {
		return false
	}
	budget.readMu.Lock()
	defer budget.readMu.Unlock()
	return budget.remaining == 0
}

// ReadHTTPInput charges bytes even on a truncated or oversized response.
func ReadHTTPInput(ctx context.Context, reader io.Reader) ([]byte, error) {
	return ReadHTTPResponse(ctx, reader, -1)
}

type responseReader struct {
	io.Reader
	eof bool
}

func (r *responseReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.eof = errors.Is(err, io.EOF)
	return n, err
}

// ReadHTTPResponse uses the transport's Content-Length or observed EOF to
// accept a response that fits exactly without reading past the job budget.
func ReadHTTPResponse(ctx context.Context, reader io.Reader, size int64) ([]byte, error) {
	budget, _ := ctx.Value(httpInputKey{}).(*httpInputBudget)
	limit := int64(HTTPResponseLimit)
	if budget != nil {
		budget.readMu.Lock()
		defer budget.readMu.Unlock()
		limit = min(limit, budget.remaining)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit == 0 {
		return nil, ErrHTTPInputLimit
	}
	if size > limit {
		if budget != nil && size > budget.remaining && budget.remaining <= HTTPResponseLimit {
			return nil, ErrHTTPInputLimit
		}
		return nil, ErrHTTPResponseLimit
	}
	stream := &responseReader{Reader: reader}
	body, err := io.ReadAll(io.LimitReader(stream, limit))
	if budget != nil {
		budget.remaining -= int64(len(body))
	}
	if err != nil {
		return nil, err
	}
	if size > 0 && int64(len(body)) < size {
		return nil, io.ErrUnexpectedEOF
	}
	if int64(len(body)) == limit && !stream.eof && int64(len(body)) != size {
		if budget != nil && budget.remaining == 0 {
			return nil, ErrHTTPInputLimit
		}
		return nil, ErrHTTPResponseLimit
	}
	return body, nil
}
