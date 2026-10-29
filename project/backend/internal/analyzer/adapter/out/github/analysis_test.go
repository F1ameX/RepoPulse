package github

import (
	"net/http"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestAnalysisPinsRevisionAndPreservesConfirmedEmptyRepository(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name   string
		status int
		body   string
		sha    *string
	}{
		{"commit", 200, `{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, &sha},
		{"empty repository", 409, `{"message":"Git Repository is empty."}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return response(200, publicMetadata, nil), nil
				}
				if r.URL.Path != "/repositories/9007199254740993/commits/main" {
					t.Fatalf("revision request: %s", r.URL)
				}
				return response(tc.status, tc.body, nil), nil
			})}, "")
			got, err := client.LookupForAnalysis(t.Context(), domain.RepositoryURL{Owner: "owner", Name: "repo"})
			if err != nil || calls != 2 || (got.CommitSHA == nil) != (tc.sha == nil) {
				t.Fatalf("repository=%+v error=%#v calls=%d", got, err, calls)
			}
			if tc.sha != nil && *got.CommitSHA != *tc.sha {
				t.Fatal("HEAD revision was not pinned")
			}
		})
	}
}
