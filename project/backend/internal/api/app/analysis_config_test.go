package app

import (
	"strings"
	"testing"
	"time"
)

const configuredVersions = `{"schema":"schema","methodology":"methodology","scoring_config":"score","collector_profile":"profile","secret_rules":"secrets","recommendation_catalog":"catalog","explanation_schema":"explanation","prompt":"prompt"}`

func TestAnalysisConfigurationRequiresExplicitDeployedVersions(t *testing.T) {
	base := map[string]string{"ANALYZER_DATABASE_URL": "postgres://localhost/repopulse", "ANALYZER_VERSIONS": configuredVersions}
	cfg, err := loadAnalysisConfig(fromMap(base))
	if err != nil || cfg.AuthTarget != "127.0.0.1:9090" || cfg.AuthTimeout != 3*time.Second || cfg.DatabaseTimeout != 5*time.Second || cfg.Versions.CollectorProfile != "profile" {
		t.Fatalf("configuration=%+v error=%#v", cfg, err)
	}
	for _, tc := range []struct{ key, value string }{
		{"ANALYZER_DATABASE_URL", ""}, {"ANALYZER_VERSIONS", ""}, {"ANALYZER_VERSIONS", `{}`},
		{"ANALYZER_VERSIONS", `{"schema":"only-one-version"}`},
		{"AUTH_GRPC_TARGET", "localhost"}, {"AUTH_GRPC_TARGET", "localhost:0"},
		{"AUTH_RPC_TIMEOUT", "0s"}, {"ANALYZER_DATABASE_TIMEOUT", "-1s"},
		{"API_ALLOWED_ORIGIN", "https://example.com/path"}, {"API_ALLOWED_ORIGIN", "null"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			values := map[string]string{"ANALYZER_DATABASE_URL": base["ANALYZER_DATABASE_URL"], "ANALYZER_VERSIONS": base["ANALYZER_VERSIONS"]}
			values[tc.key] = tc.value
			if _, err := loadAnalysisConfig(fromMap(values)); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected configuration error naming %s, got %#v", tc.key, err)
			}
		})
	}
}
