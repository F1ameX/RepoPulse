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
	if err != nil || cfg.DatabaseTimeout != 5*time.Second || cfg.Versions.CollectorProfile != "profile" {
		t.Fatalf("configuration=%+v error=%#v", cfg, err)
	}
	for _, tc := range []struct{ key, value string }{
		{"ANALYZER_DATABASE_URL", ""}, {"ANALYZER_VERSIONS", ""}, {"ANALYZER_VERSIONS", `{}`},
		{"ANALYZER_VERSIONS", `{"schema":"only-one-version"}`},
		{"ANALYZER_DATABASE_TIMEOUT", "0s"}, {"ANALYZER_DATABASE_TIMEOUT", "-1s"},
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
