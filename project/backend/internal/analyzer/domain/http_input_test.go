package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHTTPInputBudgetIsSharedBySourceContexts(t *testing.T) {
	ctx := context.WithValue(t.Context(), httpInputKey{}, &httpInputBudget{remaining: 6})
	first, cancel := context.WithCancel(WithHTTPInputBudget(ctx))
	defer cancel()
	if body, err := ReadHTTPInput(first, strings.NewReader("abc")); string(body) != "abc" || err != nil {
		t.Fatalf("first response: %q %v", body, err)
	}
	second, cancel := context.WithCancel(WithHTTPInputBudget(ctx))
	defer cancel()
	if _, err := ReadHTTPInput(second, strings.NewReader("0123")); !errors.Is(err, ErrHTTPInputLimit) || !HTTPInputExhausted(ctx) {
		t.Fatalf("second response should exhaust shared budget: %v", err)
	}
}

func TestHTTPAttemptBudgetsAcrossPhases(t *testing.T) {
	ctx := WithHTTPInputBudget(t.Context())
	for source, limit := range map[HTTPSource]int{HTTPGitHub: 20, HTTPOSV: 40, HTTPLLM: 1} {
		for i := 0; i < limit; i++ {
			// A new child context for another page/retry/phase keeps the job's counts.
			attempt, cancel := context.WithTimeout(WithHTTPInputBudget(ctx), time.Second)
			err := TakeHTTPAttempt(attempt, source)
			cancel()
			if err != nil {
				t.Fatalf("source %d attempt %d: %v", source, i+1, err)
			}
		}
		if err := HTTPAttemptAvailable(ctx, source); !errors.Is(err, ErrHTTPAttemptLimit) {
			t.Fatalf("source %d after its last attempt: %v", source, err)
		}
	}
	budget := ctx.Value(httpInputKey{}).(*httpInputBudget)
	if budget.total != 61 {
		t.Fatalf("got %d attempts", budget.total)
	}
	budget.attempts[HTTPLLM] = 0
	if err := TakeHTTPAttempt(ctx, HTTPLLM); !errors.Is(err, ErrHTTPAttemptLimit) || budget.total != 61 {
		t.Fatalf("total cap was bypassed: %v", err)
	}
}

func TestHTTPResponseBoundaryAndNoReadOverInputBudget(t *testing.T) {
	ctx := WithHTTPInputBudget(t.Context())
	tooBig := strings.NewReader(strings.Repeat("a", HTTPResponseLimit+1))
	if _, err := ReadHTTPResponse(ctx, tooBig, tooBig.Size()); !errors.Is(err, ErrHTTPResponseLimit) || int64(tooBig.Len()) != tooBig.Size() {
		t.Fatalf("oversize response was read: %v", err)
	}
	body := strings.Repeat("a", HTTPResponseLimit)
	for i := 0; i < 10; i++ {
		if data, err := ReadHTTPResponse(ctx, strings.NewReader(body), int64(len(body))); err != nil || len(data) != HTTPResponseLimit {
			t.Fatalf("boundary response %d failed: %v", i+1, err)
		}
	}
	if err := TakeHTTPAttempt(ctx, HTTPOSV); !errors.Is(err, ErrHTTPInputLimit) {
		t.Fatalf("attempt allowed after 50 MiB: %v", err)
	}
	reader := strings.NewReader("x")
	if _, err := ReadHTTPInput(ctx, reader); !errors.Is(err, ErrHTTPInputLimit) || reader.Len() != 1 {
		t.Fatalf("input was read beyond 50 MiB: %v", err)
	}
}

func TestBackoffFitsOnlyBeforeDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	if !HTTPRetryFits(ctx, time.Second) || HTTPRetryFits(ctx, time.Minute) {
		t.Fatal("backoff was not bounded by the remaining time")
	}
}
