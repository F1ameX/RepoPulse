package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
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
