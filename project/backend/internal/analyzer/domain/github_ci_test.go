package domain

import "testing"

func TestCIConfigurationComesFromCompletePinnedTree(t *testing.T) {
	git := GitData{Files: []GitFile{
		{Path: ".github/workflows/build.yaml", Mode: "100644"},
		{Path: ".github/workflows/nested/ignored.yml", Mode: "100644"},
		{Path: "Jenkinsfile", Mode: "120000"},
		{Path: ".gitlab-ci.yml", Mode: "100644"},
	}, Sources: []SourceState{{Source: "git", Scope: GitTreeScope, Status: "partial"}}}
	var ci GitHubCI
	ci.CollectConfigurations(git)
	if len(ci.ConfigPaths) != 2 || ci.ConfigurationPresent() != nil {
		t.Fatalf("partial tree: %+v", ci)
	}
	git.Sources[0].Status = "complete"
	ci.CollectConfigurations(git)
	if present := ci.ConfigurationPresent(); present == nil || !*present {
		t.Fatalf("complete tree with configurations: %+v", ci)
	}
	git.Files = nil
	ci.CollectConfigurations(git)
	if present := ci.ConfigurationPresent(); present == nil || *present {
		t.Fatalf("complete empty tree: %+v", ci)
	}
}
