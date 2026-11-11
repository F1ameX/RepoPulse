package activity

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestTreeMetricsMatchPublishedFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.json")
	if err != nil || len(paths) != 13 {
		t.Fatalf("documentation and testing fixtures: %v (%d files)", err, len(paths))
	}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var fixture struct {
				Input struct {
					Files  []string `json:"files"`
					Status string   `json:"tree_status"`
					Reason string   `json:"tree_reason"`
				} `json:"input"`
				Expected struct {
					Metrics map[string]struct {
						Raw      any       `json:"raw_value"`
						Score    *float64  `json:"normalized_score"`
						Status   string    `json:"status"`
						Reason   *string   `json:"reason"`
						Evidence *[]string `json:"evidence_paths"`
					} `json:"metrics"`
				} `json:"expected"`
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &fixture); err != nil {
				t.Fatal(err)
			}
			a := runningAnalysis(t)
			tree := domain.SourceState{Source: "git", Scope: "tree", Status: fixture.Input.Status}
			if fixture.Input.Reason != "" {
				tree.Reason = &domain.Reason{Code: fixture.Input.Reason, Message: "Сбор дерева прерван", CannotConclude: "Полнота дерева неизвестна"}
			}
			git := domain.GitData{Repository: a.Repository, Sources: []domain.SourceState{tree}}
			for _, name := range fixture.Input.Files {
				git.Files = append(git.Files, domain.GitFile{Path: name, Mode: "100644"})
			}
			output, err := NewProcessor(engine).Process(t.Context(), a, controlSession{git}, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			// Check that both new categories form a publishable report with the
			// exact same inputs as the snapshot.
			result, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, output.Snapshot, output.Report)
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			calculated := 0
			for _, category := range result.Report.Categories {
				calculated += category.Coverage.Calculated
				for _, metric := range category.Metrics {
					want, ok := fixture.Expected.Metrics[metric.Input.ID]
					if !ok {
						continue
					}
					checked++
					raw, _ := metric.Input.RawValue.MarshalJSON()
					var value any
					if err := json.Unmarshal(raw, &value); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(value, want.Raw) || metric.Input.Status != want.Status {
						t.Fatalf("%s: got raw=%s status=%s, want raw=%v status=%s", metric.Input.ID, raw, metric.Input.Status, want.Raw, want.Status)
					}
					if want.Reason == nil && metric.Input.Reason != nil ||
						want.Reason != nil && (metric.Input.Reason == nil || metric.Input.Reason.Code != *want.Reason) {
						t.Fatalf("%s: unexpected reason %+v", metric.Input.ID, metric.Input.Reason)
					}
					if want.Score == nil {
						if metric.NormalizedScore != nil {
							t.Fatalf("%s: unexpected score %g", metric.Input.ID, *metric.NormalizedScore)
						}
					} else {
						public, err := scoring.PublicScore(metric.NormalizedScore)
						if err != nil || public == nil || *public != math.Round(*want.Score*10)/10 {
							t.Fatalf("%s: incorrect score %v (%v)", metric.Input.ID, public, err)
						}
						if metric.Input.ID == "DOC_SUPPORT_SHARE" && math.Abs(*metric.NormalizedScore-value.(float64)*100) > 1e-12 {
							t.Fatal("documentation score was rounded before aggregation")
						}
					}
					if want.Evidence != nil {
						got := make([]string, 0, len(metric.Input.Evidence))
						for _, evidence := range metric.Input.Evidence {
							if evidence.Path == nil || evidence.Revision == nil || *evidence.Revision != *a.Repository.CommitSHA {
								t.Fatalf("missing pinned path evidence: %+v", evidence)
							}
							got = append(got, *evidence.Path)
						}
						expected := slices.Clone(*want.Evidence)
						slices.Sort(expected)
						if !slices.Equal(got, expected) {
							t.Fatalf("paths %v, want %v", got, expected)
						}
					}
					if strings.HasPrefix(filepath.Base(path), "TEST-06") &&
						(!metric.Input.EvidenceLimited || metric.Input.EvidenceTotal == nil || *metric.Input.EvidenceTotal != 12) {
						t.Fatal("evidence limit lost the total match count")
					}
				}
			}
			if checked != len(fixture.Expected.Metrics) || result.Report.Coverage.Total != 12 || result.Report.Coverage.Calculated != calculated {
				t.Fatalf("incomplete report: checked %d, coverage %+v", checked, result.Report.Coverage)
			}
		})
	}
}

func TestTreeIndicatorsRespectPathScopeAndFileMode(t *testing.T) {
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	a := runningAnalysis(t)
	tree := domain.SourceState{Source: "git", Scope: "tree", Status: "complete"}
	git := domain.GitData{Repository: a.Repository, Files: []domain.GitFile{
		{Path: "README.md", Mode: "120000"},
		{Path: "doc/README.rst", Mode: "100644"},
		{Path: "LICENSE", Mode: "100644"},
		{Path: ".github/LICENSE", Mode: "100644"},
		{Path: ".github/nested/CONTRIBUTING.md", Mode: "100644"},
		{Path: "vendor", Mode: "160000"},
		{Path: "src/main.go", Mode: "100644"},
		{Path: "app/tests/fake.go", Mode: "120000"},
		{Path: "app/tests/data.json", Mode: "100644"},
		{Path: "app/specifications/helper.go", Mode: "100644"},
		{Path: "app/example_spec.go", Mode: "100644"},
	}}
	inputs := engine.TreeInputs(git, tree)
	readme, _ := inputs[0].RawValue.MarshalJSON()
	tests, _ := inputs[2].RawValue.MarshalJSON()
	if string(readme) != "false" || string(tests) != "false" {
		t.Fatalf("nonmatching paths or nonregular files counted: readme=%s tests=%s", readme, tests)
	}
	support := inputs[1]
	if *support.Sample.Numerator != 1 || *support.Sample.Denominator != 3 || len(support.Evidence) != 2 ||
		*support.Evidence[0].Path != ".github/LICENSE" || *support.Evidence[1].Path != "LICENSE" {
		t.Fatalf("incorrect deduplication or evidence: %+v", support)
	}
	git.Files = []domain.GitFile{{Path: "app/test_main.kt", Mode: "100644"}}
	test := engine.TreeInputs(git, tree)[2]
	if test.Status != "not_applicable" || test.Reason.Code != "NO_SOURCE_FILES" || len(test.Evidence) != 0 {
		t.Fatalf("unsupported language must have N/A: %+v", test)
	}
}
