package git

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/secrets"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestSecretsAcrossBranchesAndDeletedHistory(t *testing.T) {
	root := t.TempDir()
	command(t, root, "init", "-b", "main")
	put := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		command(t, root, "add", "--", name)
		command(t, root, "commit", "-m", "control")
	}
	put("README.md", "clean")
	command(t, root, "checkout", "-b", "other")
	private := "-----BEGIN " + "OPENSSH PRIVATE KEY-----"
	credential := "synthetic" + "credential"
	put("old.txt", private+"\npassword = \""+credential+"\"\n")
	old := command(t, root, "rev-parse", "HEAD")
	command(t, root, "tag", "old-state")
	command(t, root, "rm", "old.txt")
	command(t, root, "commit", "-m", "removed")
	command(t, root, "checkout", "main")
	reader, err := NewReader(t.TempDir(), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{reader: reader, root: root, dir: filepath.Join(root, ".git"), ctx: context.Background()}
	session.data.Repository.CommitSHA = new(command(t, root, "rev-parse", "HEAD"))
	session.readHistory(t.Context(), "", false)
	session.data.Sources = append(session.data.Sources, source("refs", 3, "", "partial"))
	analysis := domain.Analysis{Versions: domain.Versions{SecretRules: secrets.RulesVersion}}
	result := secrets.Collect(t.Context(), analysis, session)
	input := result.Inputs[0].Value
	if input.Status != "complete" || len(input.Evidence) != 2 || input.Evidence[0].Revision == nil || *input.Evidence[0].Revision != old {
		t.Fatalf("historical matches lost: %+v", input)
	}
	body, err := json.Marshal(result)
	if err != nil || strings.Contains(string(body), credential) || strings.Contains(string(body), private) {
		t.Fatal("source content escaped scanner")
	}
	if result.Sources[0].Sample.Collected == nil || *result.Sources[0].Sample.Collected != 3 {
		t.Fatal("history coverage lost")
	}
	// An oversized historical blob stops the scan and cannot produce a complete zero.
	put("oversized.txt", strings.Repeat("a", (1<<20)+1))
	session.data.Commits = nil
	session.data.Sources = nil
	session.readHistory(t.Context(), "", false)
	session.data.Sources = append(session.data.Sources, source("refs", 3, "", "partial"))
	result = secrets.Collect(t.Context(), analysis, session)
	if result.Sources[0].Status != "partial" || result.Sources[0].Reason.Code != "SECRET_BLOB_SIZE_LIMIT" {
		t.Fatalf("size boundary not enforced: %+v", result.Sources[0])
	}
	raw, _ := result.Inputs[0].Value.RawValue.MarshalJSON()
	if string(raw) != "null" {
		t.Fatal("partial scan assigned a raw result")
	}
}
