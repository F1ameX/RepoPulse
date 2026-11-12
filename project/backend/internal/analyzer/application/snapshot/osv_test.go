package snapshot

import (
	"context"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type osvQueryFunc func(context.Context, []domain.OSVQuery) []domain.OSVResult

func (f osvQueryFunc) Query(ctx context.Context, queries []domain.OSVQuery) []domain.OSVResult {
	return f(ctx, queries)
}

func TestOSVPartialCollectionKeepsFindingsAndIndependentSources(t *testing.T) {
	a, git := accepted()
	at := a.AssessmentAt.Add(time.Second)
	latest := at.Add(time.Second)
	origin := domain.DependencyOrigin{ManifestPath: "go.mod", VersionSource: "go.mod"}
	dep := domain.Dependency{Ecosystem: "Go", Package: "example.org/module", Version: "v1.2.3",
		ManifestPath: origin.ManifestPath, VersionSource: origin.VersionSource, Origins: []domain.DependencyOrigin{origin}}
	dependencyFacts := Collection{Dependencies: []domain.Dependency{dep, dep}, Sources: []domain.SourceState{
		{Source: "git", Scope: "dependencies", Status: "complete"},
		{Source: "github", Scope: "actions", Status: "complete"},
	}}
	client := osvQueryFunc(func(_ context.Context, queries []domain.OSVQuery) []domain.OSVResult {
		if len(queries) != 1 || queries[0] != (domain.OSVQuery{Ecosystem: dep.Ecosystem, Name: dep.Package, Version: dep.Version}) {
			t.Fatalf("unexpected OSV queries: %+v", queries)
		}
		return []domain.OSVResult{{Query: queries[0], ReasonCode: "OSV_ATTEMPT_LIMIT", Pages: []domain.OSVPage{
			{VulnerabilityIDs: []string{"TEST-1"}, ObtainedAt: at},
			{VulnerabilityIDs: []string{"TEST-1"}, ObtainedAt: latest},
		}}}
	})
	osvFacts, err := collectOSV(t.Context(), client, []Collection{dependencyFacts})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Build(a, git, dependencyFacts, osvFacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Dependencies) != 1 || len(snapshot.Vulnerabilities) != 1 ||
		!snapshot.Vulnerabilities[0].ObtainedAt.Equal(latest) || snapshot.Vulnerabilities[0].Severity != nil {
		t.Fatalf("unexpected snapshot facts: %+v", snapshot.Vulnerabilities)
	}
	for _, source := range snapshot.Sources {
		if source.Source == "osv" {
			if source.Status != "partial" || source.Reason == nil || source.Reason.Code != "OSV_ATTEMPT_LIMIT" ||
				source.Sample.Total != nil || source.ObtainedAt == nil || !source.ObtainedAt.Equal(latest) {
				t.Fatalf("unexpected OSV scope: %+v", source)
			}
		} else if source.Status != "complete" {
			t.Fatalf("OSV degraded an independent scope: %+v", source)
		}
	}
}
