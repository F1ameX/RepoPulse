package git

import (
	"strings"
	"testing"
)

func TestGitIgnoresExactDuplicatesAndDropsConflictingObjects(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	_, _, refs, err := parseRefs([]byte(a + "\trefs/heads/one\n" + a + "\trefs/heads/one\n" +
		b + "\trefs/heads/two\n" + c + "\trefs/heads/two\n" + c + "\trefs/tags/v1\n"))
	if err == nil || len(refs) != 2 || refs[0].Name != "refs/heads/one" || refs[1].Name != "refs/tags/v1" {
		t.Fatalf("ref conflict not isolated: %+v, %v", refs, err)
	}
	walk := parseHistory(t.Context(), []byte("1 "+a+"\n1 "+a+"\n2 "+b+"\n3 "+b+"\n4 "+c+"\n"), 3)
	if walk.code != "GIT_HISTORY_INVALID" || len(walk.commits) != 2 || walk.commits[0].SHA != a || walk.commits[1].SHA != c {
		t.Fatalf("commit conflict not isolated: %+v", walk)
	}
}
