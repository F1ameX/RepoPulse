package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestRules(t *testing.T) {
	for _, name := range []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "OPENSSH PRIVATE KEY"} {
		if !slices.Equal(MatchTypes([]byte("-----BEGIN "+name+"-----\n")), []string{"private_key_header"}) {
			t.Fatal("header unsupported")
		}
	}
	for _, tc := range []struct {
		body  string
		match bool
	}{
		{"PaSsWoRd = \"abcdefgh\"", true},
		{"api_key: 'abcdefgh'", true},
		{"TOKEN = \"" + strings.Repeat("x", 128) + "\"", true},
		{"token = \"" + strings.Repeat("x", 129) + "\"", false},
		{"token = \"1234567\"", false},
		{"mytoken = \"abcdefgh\"", false},
		{"-----BEGIN PUBLIC KEY-----", false},
		{"-----BEGIN RSA PRIVATE KEY-----suffix", false},
	} {
		if (len(MatchTypes([]byte(tc.body))) > 0) != tc.match {
			t.Fatal("rule boundary mismatch")
		}
	}
}

type memorySession struct {
	data    domain.GitData
	body    []byte
	treeErr error
}

func (s memorySession) Data() domain.GitData                             { return s.data }
func (s memorySession) ReadBlob(context.Context, string) ([]byte, error) { return s.body, nil }
func (s memorySession) Close() error                                     { return nil }
func (s memorySession) HistoryTree(context.Context, string) ([]domain.GitFile, error) {
	return []domain.GitFile{{Path: "config.txt", ObjectSHA: strings.Repeat("b", 40), Mode: "100644", Size: int64(len(s.body))}}, s.treeErr
}

func TestCompletenessAndRedaction(t *testing.T) {
	analysis := domain.Analysis{Versions: domain.Versions{SecretRules: RulesVersion}}
	for _, tc := range []struct {
		name, history string
		body          []byte
		err           error
		cancel        bool
		want          string
	}{
		{"clean", "complete", []byte("clean"), nil, false, "complete"},
		{"binary", "complete", []byte{0, 1}, nil, false, "partial"},
		{"unreadable", "complete", []byte{255}, nil, false, "partial"},
		{"truncated history", "partial", []byte("clean"), nil, false, "partial"},
		{"missing tree", "complete", nil, errors.New("private upstream details"), false, "partial"},
		{"time limit", "complete", nil, nil, true, "partial"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := memorySession{body: tc.body, treeErr: tc.err, data: domain.GitData{
				Commits: []domain.GitCommit{{SHA: strings.Repeat("a", 40)}},
				Sources: []domain.SourceState{{Source: "git", Scope: "refs", Status: "complete"}, {Source: "git", Scope: "history", Status: tc.history}},
			}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			result := Collect(ctx, analysis, session)
			input := result.Inputs[0].Value
			if input.Status != tc.want || (tc.want == "partial" && input.Reason == nil) {
				t.Fatalf("%+v", input)
			}
			if err := input.Validate(); err != nil {
				t.Fatal(err)
			}
			raw, _ := input.RawValue.MarshalJSON()
			if tc.want == "complete" && string(raw) != "0" || tc.want == "partial" && string(raw) != "null" {
				t.Fatalf("raw %s", raw)
			}
			body, _ := json.Marshal(result)
			if strings.Contains(string(body), "private upstream details") {
				t.Fatal("error content escaped")
			}
		})
	}
}
