package git

import (
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestPinnedTreeDistinguishesExactLimitFromTruncation(t *testing.T) {
	metadata, server, _ := fixture(t)
	for _, tc := range []struct {
		limit        int
		status, code string
	}{{4, "complete", ""}, {3, "partial", "GIT_PATH_LIMIT"}} {
		limits := DefaultLimits()
		limits.Paths = tc.limit
		session, err := readerFor(t, server, limits, false).Open(t.Context(), metadata)
		if err != nil {
			t.Fatal(err)
		}
		data := session.Data()
		if err := session.Close(); err != nil {
			t.Fatal(err)
		}
		var state domain.SourceState
		for _, source := range data.Sources {
			if source.Scope == domain.GitTreeScope {
				state = source
			}
		}
		if len(data.Files) != tc.limit || state.Status != tc.status || state.Sample.Collected == nil || *state.Sample.Collected != tc.limit {
			t.Fatalf("limit %d: tree has %d files and state %+v", tc.limit, len(data.Files), state)
		}
		if tc.code == "" {
			if state.Reason != nil || state.Sample.Total == nil || *state.Sample.Total != tc.limit {
				t.Fatalf("complete tree: %+v", state)
			}
		} else if state.Reason == nil || state.Reason.Code != tc.code || state.Sample.Total != nil || state.Fingerprint != nil {
			t.Fatalf("truncated tree: %+v", state)
		}
	}
}

func TestTreeKeepsNestedPathsButNotAnInterruptedRecord(t *testing.T) {
	for _, tc := range []struct {
		name, suffix string
		withSizes    bool
		size         int64
	}{{"with sizes", " 17", true, 17}, {"paths only", "", false, -1}} {
		t.Run(tc.name, func(t *testing.T) {
			header := "100644 blob " + strings.Repeat("a", 40) + tc.suffix + "\t"
			out := treeOutput{remaining: 1000, pathLimit: 5, withSizes: tc.withSizes}
			for _, chunk := range []string{header + "docs/SEC", "URITY.md\x00" + header + "src/check_test.go\x00", header + "src/fake_test.go"} {
				if _, err := out.Write([]byte(chunk)); err != nil {
					t.Fatal(err)
				}
			}
			if len(out.files) != 2 || out.files[0].Path != "docs/SECURITY.md" || out.files[1].Path != "src/check_test.go" ||
				out.files[0].Size != tc.size || len(out.pending) == 0 {
				t.Fatalf("incorrect complete paths: %+v", out.files)
			}
		})
	}
}
