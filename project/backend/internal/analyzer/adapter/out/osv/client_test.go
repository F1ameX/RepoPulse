package osv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestQuerySendsExactCoordinatesAndKeepsPagesAfterFailure(t *testing.T) {
	queries := []domain.OSVQuery{
		{Ecosystem: "npm", Name: "@scope/pkg", Version: "1.2.3"},
		{Ecosystem: "Maven", Name: "example:lib", Version: "2.0.0"},
		{Ecosystem: "Go", Name: "example.org/mod/v2", Version: "v2.3.0-rc.1"},
	}
	pages := []struct {
		index  int
		token  string
		status int
		body   string
	}{
		{0, "", 200, `{"vulns":[{"id":"TEST-1","summary":"retained in memory"}],"next_page_token":"second"}`},
		{0, "second", 429, ""},
		{0, "second", 200, `{"next_page_token":"third"}`},
		{0, "third", 400, `{"message":"ignored upstream text"}`},
		{1, "", 200, `{"vulns":[{"id":"TEST-2"}]}`},
		{2, "", 200, `{}`},
	}
	calls := 0
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls >= len(pages) || r.Method != http.MethodPost || r.URL.String() != queryURL {
			t.Fatalf("unexpected HTTP request %s %s", r.Method, r.URL)
		}
		page := pages[calls]
		calls++
		defer r.Body.Close()
		var got queryBody
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		want := queries[page.index]
		if got.Package.Name != want.Name || got.Package.Ecosystem != want.Ecosystem || got.Version != want.Version || got.PageToken != page.token {
			t.Fatalf("request %d: %+v", calls, got)
		}
		res := response(page.status, page.body)
		if page.status == http.StatusTooManyRequests {
			res.Header.Set("Retry-After", "0")
		}
		return res, nil
	})})
	results := client.Query(t.Context(), queries)
	if calls != len(pages) || len(results) != 3 || results[0].ReasonCode != "OSV_HTTP_ERROR" || len(results[0].Pages) != 2 {
		t.Fatalf("partial result or retry was lost: calls=%d results=%+v", calls, results)
	}
	for i, want := range []string{"TEST-1", "TEST-2"} {
		page := results[i].Pages[0]
		if len(page.VulnerabilityIDs) != 1 || page.VulnerabilityIDs[0] != want || page.ObtainedAt.IsZero() || !json.Valid(page.Body) {
			t.Fatalf("response was not retained: %+v", page)
		}
	}
	if results[1].ReasonCode != "" || results[2].ReasonCode != "" || len(results[2].Pages) != 1 || len(results[2].Pages[0].VulnerabilityIDs) != 0 {
		t.Fatalf("later packages did not finish independently: %+v", results)
	}
}

func TestQuerySharesFortyAttemptsAcrossProfilesPagesAndRetries(t *testing.T) {
	queries := []domain.OSVQuery{
		{Ecosystem: "npm", Name: "pkg", Version: "1.0.0"},
		{Ecosystem: "Maven", Name: "example:lib", Version: "2.0.0"},
		{Ecosystem: "Go", Name: "example.org/mod", Version: "v1.0.0"},
	}
	calls := 0
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		defer r.Body.Close()
		switch calls {
		case 1:
			return response(200, "{}"), nil
		case 2:
			res := response(429, "")
			res.Header.Set("Retry-After", "0")
			return res, nil
		case 3:
			return response(200, "{}"), nil
		case 40:
			// The last attempt fails. It must not trigger another retry.
			res := response(429, "")
			res.Header.Set("Retry-After", "0")
			return res, nil
		}
		if calls > 40 {
			t.Fatal("issued HTTP request 41")
		}
		return response(200, fmt.Sprintf(`{"next_page_token":"page-%d"}`, calls)), nil
	})})
	results := client.Query(t.Context(), queries)
	if calls != 40 || results[0].ReasonCode != "" || results[1].ReasonCode != "" ||
		results[2].ReasonCode != "OSV_ATTEMPT_LIMIT" || len(results[2].Pages) != 36 {
		t.Fatalf("budget was not shared: calls=%d results=%+v", calls, results)
	}
}
