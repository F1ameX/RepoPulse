package activity

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type mavenSession struct {
	controlSession
	manifest string
}

func (s mavenSession) ReadBlob(_ context.Context, object string) ([]byte, error) {
	if object != "pom" {
		return nil, domain.ErrInvalidData
	}
	return []byte(s.manifest), nil
}

func TestRuntimeOSVRetainsFindingsWithIncompleteDependencyCoverage(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name         string
		dependencies int
		unresolved   bool
		wantQueries  int
		reason       string
	}{
		{"unknown version", 2, true, 1, "DEPENDENCY_PROFILE_INCOMPLETE"},
		{"dependency limit", 21, false, 20, "DEPENDENCY_LIMIT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pom strings.Builder
			pom.WriteString("<project><dependencies>")
			for i := 0; i < tc.dependencies; i++ {
				version := "1.0"
				if tc.unresolved && i == tc.dependencies-1 {
					version = ""
				}
				fmt.Fprintf(&pom, "<dependency><groupId>example</groupId><artifactId>lib%02d</artifactId><version>%s</version></dependency>", i, version)
			}
			pom.WriteString("</dependencies></project>")

			git := domain.GitData{
				Repository: a.Repository,
				Files:      []domain.GitFile{{Path: "pom.xml", ObjectSHA: "pom", Mode: "100644"}},
				Sources:    []domain.SourceState{{Source: "git", Scope: domain.GitTreeScope, Status: "complete"}},
			}
			const advisory = "TEST-OSV-1"
			client := fixedOSV(func(_ context.Context, queries []domain.OSVQuery) []domain.OSVResult {
				if len(queries) != tc.wantQueries {
					t.Fatalf("got %d OSV queries, want %d", len(queries), tc.wantQueries)
				}
				results := make([]domain.OSVResult, 0, len(queries))
				for i, query := range queries {
					want := domain.OSVQuery{Ecosystem: "Maven", Name: fmt.Sprintf("example:lib%02d", i), Version: "1.0"}
					if query != want {
						t.Fatalf("got query %+v, want %+v", query, want)
					}
					page := domain.OSVPage{ObtainedAt: a.AssessmentAt}
					if i == 0 {
						page.VulnerabilityIDs = []string{advisory}
						page.Severity = map[string]string{advisory: "high"}
					}
					results = append(results, domain.OSVResult{Query: query, Pages: []domain.OSVPage{page}})
				}
				return results
			})

			out, err := NewProcessorWithSources(engine, nil, client).Process(t.Context(), a, mavenSession{controlSession{git}, pom.String()}, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, out.Snapshot, out.Report); err != nil {
				t.Fatal(err)
			}
			metric := out.Report.Categories[3].Metrics[1]
			raw, err := metric.Input.RawValue.MarshalJSON()
			if err != nil || metric.Input.ID != "SEC_VULN_RISK" || string(raw) != "null" ||
				metric.NormalizedScore != nil || metric.Input.Status != "partial" ||
				metric.Input.Reason == nil || metric.Input.Reason.Code != tc.reason {
				t.Fatalf("expected partial risk with no score and reason %s: %+v, raw %s", tc.reason, metric, raw)
			}
			findings := out.Report.Findings
			if len(out.Snapshot.Vulnerabilities) != 1 || len(findings) != 1 || len(findings[0].Vulnerabilities) != 1 {
				t.Fatalf("confirmed finding missing from snapshot or report: %+v", findings)
			}
			vulnerability := findings[0].Vulnerabilities[0]
			if vulnerability.ID != advisory || vulnerability.Package != "example:lib00" ||
				vulnerability.Severity == nil || *vulnerability.Severity != "high" {
				t.Fatalf("unexpected confirmed finding: %+v", vulnerability)
			}
		})
	}
}
