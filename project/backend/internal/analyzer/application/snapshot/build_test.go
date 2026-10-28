package snapshot

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func ptr[T any](value T) *T { return &value }

func scalar(t *testing.T, raw string) domain.Scalar {
	t.Helper()
	var value domain.Scalar
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func accepted() (domain.Analysis, domain.GitData) {
	at := time.Date(2026, 10, 20, 12, 0, 0, 123456000, time.UTC)
	a := domain.Analysis{ID: domain.NewID(), SubmissionID: domain.NewID(), RequestID: domain.NewID(),
		Owner: domain.Owner{Kind: "guest", ID: domain.NewID()}, AssessmentAt: at, CreatedAt: at,
		Window: domain.Window{From: at.Add(-140 * 24 * time.Hour), Until: at}, Status: "running", StartedAt: &at,
		Repository: domain.RepositoryMetadata{Platform: "github", GitHubID: "42", Owner: "owner", Name: "repo",
			CanonicalURL: "https://github.com/owner/repo", DefaultBranch: ptr("main"), CommitSHA: ptr(strings.Repeat("a", 40))},
		Versions: domain.Versions{Schema: "mvp-contract-1.1.0", Methodology: "rhs-mvp-1.1.0", ScoringConfig: "rhs-mvp-1.1.0",
			CollectorProfile: "github-mvp-1.1.0", SecretRules: "secrets-mvp-1.0.0", RecommendationCatalog: "recommendations-mvp-1.0.0",
			ExplanationSchema: "catalog-selection-1.0.0", Prompt: "catalog-selection-1.0.0"}}
	git := domain.GitData{Repository: a.Repository, Refs: []domain.GitRef{{Name: "refs/heads/main", ObjectSHA: *a.Repository.CommitSHA}},
		Sources: []domain.SourceState{{Source: "git", Scope: "tree", Status: "complete", ObtainedAt: ptr(at.Add(time.Second)),
			Sample: domain.Sample{Collected: ptr(0), Total: ptr(0)}}}}
	return a, git
}

func metric(t *testing.T, id, unit, raw string, source Source) Input {
	return Input{Value: domain.MetricInput{ID: id, Unit: unit, RawValue: scalar(t, raw), Status: "complete"}, Sources: []Source{source}}
}

func find(t *testing.T, data domain.SnapshotData, id string) domain.MetricInput {
	t.Helper()
	for _, input := range data.Inputs {
		if input.ID == id {
			return input
		}
	}
	t.Fatalf("missing input %s", id)
	return domain.MetricInput{}
}

func TestBuildPreservesValuesObservationsAndUnknowns(t *testing.T) {
	a, git := accepted()
	obtained := a.AssessmentAt.Add(2*time.Second + 987*time.Nanosecond).In(time.FixedZone("source", 7*3600))
	observed := a.AssessmentAt.Add(-48 * time.Hour)
	collection := Collection{Sources: []domain.SourceState{{Source: "github", Scope: "actions", Status: "complete", ObtainedAt: &obtained,
		Sample: domain.Sample{Collected: ptr(10), Total: ptr(10)}}}, Inputs: []Input{
		metric(t, "DOC_README", "bool", "false", Source{"git", "tree"}),
		metric(t, "DOC_SUPPORT_SHARE", "ratio", "0", Source{"git", "tree"}),
		metric(t, "CI_SUCCESS_RATE", "ratio", "0.75", Source{"github", "actions"}),
	}}
	collection.Inputs[2].Value.Sample = domain.Sample{Collected: ptr(10), Numerator: ptr(6), Denominator: ptr(8), Excluded: ptr(2)}
	collection.Inputs[2].Value.Exclusions = []domain.ExcludedCount{{Reason: "cancelled", Count: 2}}
	collection.Inputs[2].Value.ObservedAt = &observed
	data, err := Build(a, git, collection)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Inputs) != 12 {
		t.Fatal("not all metric states were preserved")
	}
	for id, want := range map[string]string{"DOC_README": "false", "DOC_SUPPORT_SHARE": "0", "SEC_SECRETS": "null"} {
		input := find(t, data, id)
		raw, _ := json.Marshal(input.RawValue)
		if string(raw) != want || input.Series == nil || input.Evidence == nil || input.Exclusions == nil {
			t.Fatalf("%s: %+v raw=%s", id, input, raw)
		}
	}
	missing := find(t, data, "ACT_DAYS_SINCE_LAST")
	if missing.Status != "unavailable" || missing.ObservedAt != nil || missing.Sample.Collected != nil || missing.EvidenceTotal != nil || missing.Reason.Code != "INPUT_NOT_COLLECTED" {
		t.Fatal("missing data was replaced with an observation/count")
	}
	ci := find(t, data, "CI_SUCCESS_RATE")
	if !reflect.DeepEqual(*ci.Window, a.Window) || !ci.ObservedAt.Equal(observed) || *ci.Sample.Numerator != 6 || *ci.Sample.Denominator != 8 || ci.Exclusions[0].Reason != "cancelled" {
		t.Fatal("window, actual observation, sample or exclusions lost")
	}
	if !data.Sources[1].ObtainedAt.Equal(domain.UTC(obtained)) || data.Sources[1].ObtainedAt.Location() != time.UTC {
		t.Fatal("source acquisition instant was replaced")
	}
	// No caller-owned pointer or nested list remains attached to the snapshot.
	*collection.Inputs[2].Value.Sample.Numerator = 100
	collection.Inputs[2].Value.Exclusions[0].Count = 100
	*git.Sources[0].Sample.Collected = 100
	git.Refs[0].ObjectSHA = strings.Repeat("b", 40)
	if *ci.Sample.Numerator != 6 || ci.Exclusions[0].Count != 2 || *data.Sources[0].Sample.Collected != 0 || data.Refs[0].ObjectSHA != *a.Repository.CommitSHA {
		t.Fatal("snapshot aliases collected facts")
	}
}

func TestBuildKeepsUnknownTimeAndPartialSource(t *testing.T) {
	a, git := accepted()
	git.Sources[0].Status, git.Sources[0].ObtainedAt = "partial", nil
	git.Sources[0].Reason = &domain.Reason{Code: "GIT_PATH_LIMIT", Message: "Лимит путей", CannotConclude: "Полное дерево неизвестно"}
	git.Sources[0].Sample.Total = nil
	data, err := Build(a, git, Collection{Inputs: []Input{metric(t, "DOC_README", "bool", "true", Source{"git", "tree"})}})
	if err != nil {
		t.Fatal(err)
	}
	input := find(t, data, "DOC_README")
	if input.Status != "partial" || input.Reason.Code != "GIT_PATH_LIMIT" || data.Sources[0].ObtainedAt != nil || data.Sources[0].Sample.Total != nil {
		t.Fatal("partial/unknown source was advertised as complete")
	}
}

func TestBuildRejectsMixedAnalysesAndUnsafeFacts(t *testing.T) {
	for _, name := range []string{"repository", "window", "duplicate input", "duplicate source", "source missing", "source unavailable", "unknown metric", "wrong unit", "raw secret", "unsafe URL", "zero observation", "invalid sample"} {
		t.Run(name, func(t *testing.T) {
			a, git := accepted()
			input := metric(t, "DOC_README", "bool", "false", Source{"git", "tree"})
			collection := Collection{Inputs: []Input{input}}
			switch name {
			case "repository":
				git.Repository.GitHubID = "43"
			case "window":
				collection.Inputs[0].Value.Window = &domain.Window{From: a.Window.From.Add(time.Hour), Until: a.Window.Until}
			case "duplicate input":
				collection.Inputs = append(collection.Inputs, input)
			case "duplicate source":
				collection.Sources = git.Sources
			case "source missing":
				collection.Inputs[0].Sources = []Source{{"github", "actions"}}
			case "source unavailable":
				git.Sources[0].Status = "unavailable"
			case "unknown metric":
				collection.Inputs[0].Value.ID = "OTHER"
			case "wrong unit":
				collection.Inputs[0].Value.Unit = "percent"
			case "raw secret":
				collection.Inputs[0].Value.ID, collection.Inputs[0].Value.Unit = "SEC_SECRETS", "secret_scan"
				collection.Inputs[0].Value.RawValue = scalar(t, `"password=must-not-persist"`)
			case "unsafe URL":
				collection.Inputs[0].Value.Evidence = []domain.Evidence{{ID: "unsafe", Source: "git", URL: ptr("https://user:password@github.com/owner/repo")}}
			case "zero observation":
				collection.Inputs[0].Value.ObservedAt = &time.Time{}
			case "invalid sample":
				collection.Inputs[0].Value.Sample.Collected = ptr(-1)
			}
			if _, err := Build(a, git, collection); !errors.Is(err, domain.ErrInvalidData) {
				t.Fatalf("accepted %s: %v", name, err)
			}
		})
	}
}

func TestBuildSecretEvidenceRetainsOnlySafeDescription(t *testing.T) {
	a, git := accepted()
	input := metric(t, "SEC_SECRETS", "secret_scan", "50", Source{"git", "tree"})
	input.Value.Evidence = []domain.Evidence{{ID: "secret-1", Source: "git", Kind: "secret_match", SecretMatchType: ptr("suspicious_assignment"),
		Description: "password=must-not-persist", Path: ptr("config.txt"), Revision: a.Repository.CommitSHA}}
	data, err := Build(a, git, Collection{Inputs: []Input{input}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(data)
	if strings.Contains(string(encoded), "must-not-persist") || !strings.Contains(string(encoded), "Potential secret: suspicious_assignment") {
		t.Fatal("unsafe description crossed the snapshot boundary")
	}
}

func dependency(path string) domain.Dependency {
	return domain.Dependency{Ecosystem: "Go", Package: "example.org/module", Version: "v1.2.3", ManifestPath: path,
		VersionSource: "go_require_declaration", Origins: []domain.DependencyOrigin{{ManifestPath: path, VersionSource: "go_require_declaration"}}}
}

func TestBuildDeduplicatesDependenciesWithoutLosingOriginsOrOSVUnknowns(t *testing.T) {
	a, git := accepted()
	d1, d2 := dependency("go.mod"), dependency("nested/go.mod")
	v := domain.Vulnerability{ID: "OSV-1", Ecosystem: "Go", Package: d1.Package, Version: d1.Version, ObtainedAt: a.AssessmentAt}
	left := Collection{Dependencies: []domain.Dependency{d1}, Vulnerabilities: []domain.Vulnerability{v}}
	right := Collection{Dependencies: []domain.Dependency{d2, d1}, Vulnerabilities: []domain.Vulnerability{v}}
	data, err := Build(a, git, left, right)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := Build(a, git, right, left)
	if err != nil || !reflect.DeepEqual(data, reversed) {
		t.Fatal("assembly depends on producer ordering")
	}
	if len(data.Dependencies) != 1 || len(data.Dependencies[0].Origins) != 2 || len(data.Vulnerabilities) != 1 || data.Vulnerabilities[0].Severity != nil || !data.Vulnerabilities[0].ObtainedAt.Equal(v.ObtainedAt) {
		t.Fatal("origins or original OSV facts lost")
	}
	for i := range 21 {
		d := dependency("go.mod")
		d.Package += strings.Repeat("x", i)
		left.Dependencies = append(left.Dependencies, d)
	}
	if _, err := Build(a, git, left); err == nil {
		t.Fatal("dependency profile silently exceeded 20")
	}
	v.Severity = ptr("high")
	right.Vulnerabilities = []domain.Vulnerability{v}
	left.Dependencies = []domain.Dependency{d1}
	if _, err := Build(a, git, left, right); err == nil {
		t.Fatal("conflicting OSV severity was silently selected")
	}
}
