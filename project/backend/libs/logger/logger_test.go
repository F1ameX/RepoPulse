package logger_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/libs/logger"
)

func TestWithPreservesParentAndFollowsDerivedContexts(t *testing.T) {
	first := slog.New(slog.NewJSONHandler(io.Discard, nil))
	second := first.With("request_id", "request-1")
	parent := logger.With(context.Background(), first)
	child := logger.With(parent, second)
	derived, cancel := context.WithCancel(child)
	cancel()
	if logger.From(parent) != first {
		t.Fatal("With changed the parent logger")
	}
	if logger.From(child) != second || logger.From(derived) != second {
		t.Fatal("derived contexts lost their logger")
	}
}

func TestFromFallsBackToDefault(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"absent": context.Background(),
		"nil":    logger.With(context.Background(), nil),
	} {
		t.Run(name, func(t *testing.T) {
			if got := logger.From(ctx); got != slog.Default() {
				t.Fatalf("got %p, want the default logger %p", got, slog.Default())
			}
		})
	}
}
