package command

import (
	"log/slog"
	"testing"
)

func TestLogLevelConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		set         bool
		want        slog.Level
		wantError   bool
	}{
		{name: "default", want: slog.LevelInfo},
		{name: "debug", value: " DEBUG ", set: true, want: slog.LevelDebug},
		{name: "info", value: "info", set: true, want: slog.LevelInfo},
		{name: "warn", value: "warn", set: true, want: slog.LevelWarn},
		{name: "error", value: "error", set: true, want: slog.LevelError},
		{name: "empty", set: true, wantError: true},
		{name: "invalid", value: "trace", set: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadLogLevel(func(key string) (string, bool) {
				if key != "LOG_LEVEL" {
					t.Fatalf("unexpected setting: %s", key)
				}
				return tc.value, tc.set
			})
			if (err != nil) != tc.wantError || !tc.wantError && got != tc.want {
				t.Fatalf("got level=%v error=%v, want level=%v error=%v", got, err, tc.want, tc.wantError)
			}
		})
	}
}
