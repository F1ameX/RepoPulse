package app

import "testing"

func TestReportConfigRequiresStorageAndValidBounds(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		fail             bool
	}{
		{"missing database", "REPORT_DATABASE_URL", "", true},
		{"default", "REPORT_DATABASE_URL", "postgres://localhost/repopulse", false},
		{"invalid address", "REPORT_GRPC_ADDR", "localhost:0", true},
		{"invalid timeout", "REPORT_DATABASE_TIMEOUT", "0s", true},
		{"invalid shutdown", "SHUTDOWN_TIMEOUT", "-1s", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"REPORT_DATABASE_URL": "postgres://localhost/repopulse"}
			env[tc.key] = tc.value
			cfg, err := loadConfig(func(key string) (string, bool) { v, ok := env[key]; return v, ok })
			if (err != nil) != tc.fail {
				t.Fatalf("config=%+v error=%v", cfg, err)
			}
		})
	}
}
