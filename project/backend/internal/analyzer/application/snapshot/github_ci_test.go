package snapshot

import (
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestUnavailableGitHubCIPreservesGitAndOSVInSnapshot(t *testing.T) {
	a, git := accepted()
	at := a.AssessmentAt.Add(time.Second)
	dep := domain.Dependency{Ecosystem: "Go", Package: "example.org/module", Version: "v1.2.3",
		ManifestPath: "go.mod", VersionSource: "require",
		Origins: []domain.DependencyOrigin{{ManifestPath: "go.mod", VersionSource: "require"}}}
	reason := &domain.Reason{Code: "GITHUB_CI_ACCESS_UNAVAILABLE", Message: "Access unavailable", CannotConclude: "CI state is unknown"}
	ci := &domain.GitHubCI{Attempts: []domain.ActionsAttempt{}, Rules: []domain.BranchRule{}}
	data, err := Build(a, git, Collection{GitHubCI: ci, Sources: []domain.SourceState{
		{Source: "github", Scope: domain.ActionsScope, Status: "unavailable", Reason: reason},
		{Source: "github", Scope: domain.BranchRulesScope, Status: "unavailable", Reason: reason},
	}}, Collection{
		Sources:      []domain.SourceState{{Source: "osv", Scope: domain.OSVScope, Status: "complete", ObtainedAt: &at}},
		Dependencies: []domain.Dependency{dep},
		Vulnerabilities: []domain.Vulnerability{
			{ID: "OSV-1", Ecosystem: dep.Ecosystem, Package: dep.Package, Version: dep.Version, ObtainedAt: at},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if data.GitHubCI == nil || data.GitHubCI == ci || len(data.Sources) != 4 || len(data.Vulnerabilities) != 1 {
		t.Fatalf("lost independent data or retained mutable inputs: %+v", data)
	}
	for _, source := range data.Sources {
		if source.Source != "github" && source.Status != "complete" {
			t.Fatalf("GitHub affected an independent source: %+v", source)
		}
	}
}
