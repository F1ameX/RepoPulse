package activity

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestTreeMetricsMatchPublishedFixtures(t *testing.T) {
	// DOC-01–07 and TEST-01–06 from the v1.0.1 fixtures in https://ai.nsu.ru/documents/933.
	type expectation struct {
		id     string
		raw    any
		score  float64
		status string
		reason string
		paths  []string
	}
	documentation := func(readme bool, readmeScore, share, shareScore float64) []expectation {
		return []expectation{
			{id: "DOC_README", raw: readme, score: readmeScore},
			{id: "DOC_SUPPORT_SHARE", raw: share, score: shareScore},
		}
	}
	const treeInterrupted = "GIT_TREE_INTERRUPTED"
	cappedPaths := make([]string, 12)
	for i := range cappedPaths {
		cappedPaths[i] = fmt.Sprintf("tests/test_case_%02d.py", i+1)
	}
	cases := []struct {
		name          string
		files         []string
		partial       bool
		evidenceTotal int
		want          []expectation
	}{
		{
			name: "DOC-01-readme-present", files: []string{"README.md", "src/main.go"},
			want: documentation(true, 100, 0, 0),
		},
		{
			name: "DOC-02-no-readme", files: []string{"src/main.py", "src/utils.py"},
			want: documentation(false, 0, 0, 0),
		},
		{
			name: "DOC-03-duplicate-license", files: []string{"LICENSE", ".github/LICENSE", "CONTRIBUTING.md"},
			want: documentation(false, 0, 2.0/3, 200.0/3),
		},
		{
			name:  "DOC-04-all-supporting-documents",
			files: []string{"LICENSE", "CONTRIBUTING.md", "CODE_OF_CONDUCT.md", ".github/CONTRIBUTING.md"},
			want:  documentation(false, 0, 1, 100),
		},
		{
			name: "DOC-05-case-insensitive-readme", files: []string{"readme.RST", "src/main.py"},
			want: documentation(true, 100, 0, 0),
		},
		{
			name: "DOC-06-incomplete-tree", files: []string{"README.md", "LICENSE"}, partial: true,
			want: []expectation{
				{id: "DOC_README", status: "partial", reason: treeInterrupted},
				{id: "DOC_SUPPORT_SHARE", status: "partial", reason: treeInterrupted},
			},
		},
		{
			name: "DOC-07-one-supporting-document", files: []string{"src/main.py", "LICENSE", ".github/LICENSE"},
			want: documentation(false, 0, 1.0/3, 100.0/3),
		},
		{
			name: "TEST-01-test-found", files: []string{"src/main.py", "tests/test_main.py"},
			want: []expectation{{id: "TEST_PRESENCE", raw: true, score: 100, paths: []string{"tests/test_main.py"}}},
		},
		{
			name: "TEST-02-no-tests", files: []string{"src/main.py", "src/utils.py"},
			want: []expectation{{id: "TEST_PRESENCE", raw: false}},
		},
		{
			name: "TEST-03-no-source", files: []string{"README.md", "LICENSE"},
			want: []expectation{{id: "TEST_PRESENCE", status: "not_applicable", reason: "NO_SOURCE_FILES"}},
		},
		{
			name: "TEST-04-incomplete-tree", files: []string{"src/main.py"}, partial: true,
			want: []expectation{{id: "TEST_PRESENCE", status: "partial", reason: treeInterrupted}},
		},
		{
			name:  "TEST-05-path-patterns",
			files: []string{"src/app.js", "src/app.test.js", "lib/util_spec.py", "test_api.py", "__tests__/button.jsx", "spec/router_spec.rb"},
			want: []expectation{{
				id: "TEST_PRESENCE", raw: true, score: 100,
				paths: []string{"__tests__/button.jsx", "spec/router_spec.rb", "src/app.test.js", "test_api.py"},
			}},
		},
		{
			name: "TEST-06-evidence-limit", files: append([]string{"src/app.py"}, cappedPaths...), evidenceTotal: 12,
			want: []expectation{{id: "TEST_PRESENCE", raw: true, score: 100, paths: cappedPaths[:10]}},
		},
	}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := runningAnalysis(t)
			tree := domain.SourceState{Source: "git", Scope: "tree", Status: "complete"}
			if tc.partial {
				tree.Status = "partial"
				tree.Reason = &domain.Reason{Code: treeInterrupted, Message: "Сбор дерева прерван", CannotConclude: "Полнота дерева неизвестна"}
			}
			git := domain.GitData{Repository: a.Repository, Sources: []domain.SourceState{tree}}
			for _, name := range tc.files {
				git.Files = append(git.Files, domain.GitFile{Path: name, Mode: "100644"})
			}
			output, err := NewProcessor(engine).Process(t.Context(), a, controlSession{git}, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			result, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, output.Snapshot, output.Report)
			if err != nil {
				t.Fatal(err)
			}
			metrics := map[string]domain.Metric{}
			calculated := 0
			for _, category := range result.Report.Categories {
				calculated += category.Coverage.Calculated
				for _, metric := range category.Metrics {
					metrics[metric.Input.ID] = metric
				}
			}
			if result.Report.Coverage.Total != 12 || result.Report.Coverage.Calculated != calculated {
				t.Fatalf("incorrect report coverage: %+v", result.Report.Coverage)
			}
			for _, want := range tc.want {
				metric, ok := metrics[want.id]
				if !ok {
					t.Fatalf("missing %s", want.id)
				}
				status := want.status
				if status == "" {
					status = "complete"
				}
				raw, _ := metric.Input.RawValue.MarshalJSON()
				expectedRaw, err := json.Marshal(want.raw)
				if err != nil {
					t.Fatal(err)
				}
				if string(raw) != string(expectedRaw) || metric.Input.Status != status {
					t.Fatalf("%s: got raw=%s status=%s, want raw=%s status=%s", want.id, raw, metric.Input.Status, expectedRaw, status)
				}
				reason := metric.Input.Reason
				if want.reason == "" && reason != nil || want.reason != "" && (reason == nil || reason.Code != want.reason) {
					t.Fatalf("%s: unexpected reason %+v", want.id, reason)
				}
				score := metric.NormalizedScore
				if want.raw == nil {
					if score != nil {
						t.Fatalf("%s: unexpected score %g", want.id, *score)
					}
				} else if score == nil || math.Abs(*score-want.score) > 1e-12 {
					t.Fatalf("%s: incorrect score, want %g", want.id, want.score)
				}
				if want.id != "TEST_PRESENCE" {
					continue
				}
				var paths []string
				for _, evidence := range metric.Input.Evidence {
					if evidence.Path == nil || evidence.Revision == nil || *evidence.Revision != *a.Repository.CommitSHA {
						t.Fatalf("missing pinned path evidence: %+v", evidence)
					}
					paths = append(paths, *evidence.Path)
				}
				if !slices.Equal(paths, want.paths) {
					t.Fatalf("paths %v, want %v", paths, want.paths)
				}
				if tc.evidenceTotal > 0 && (!metric.Input.EvidenceLimited || metric.Input.EvidenceTotal == nil || *metric.Input.EvidenceTotal != tc.evidenceTotal) {
					t.Fatal("evidence limit lost the total match count")
				}
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
