package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRepositoryURL(t *testing.T) {
	for _, tc := range []struct {
		input, canonical string
		failure          error
	}{
		{" https://GitHub.com/Owner/Repo.git/ ", "https://github.com/owner/repo", nil},
		{"HTTPS://GITHUB.COM/Owner/Repo", "https://github.com/owner/repo", nil},
		{"https://github.com/a-b/._repo-1", "https://github.com/a-b/._repo-1", nil},
		{"https://github.com/a/repo.git.git", "https://github.com/a/repo.git", nil},
		{"", "", ErrURLRequired}, {"   ", "", ErrURLRequired},
		{"\t", "", ErrInvalidURL}, {"\nhttps://github.com/a/b", "", ErrInvalidURL},
		{"https://github.com/a/b\r", "", ErrInvalidURL},
		{"https://github.com/a/b c", "", ErrInvalidURL},
		{"https://github.com/a/репо", "", ErrInvalidURL},
		{"https://github.com/a/\x7f", "", ErrInvalidURL},
		{"https://github.com/a/b\\c", "", ErrInvalidURL},
		{"http://github.com/a/b", "", ErrInvalidURL},
		{"ssh://github.com/a/b", "", ErrInvalidURL},
		{"git@github.com:a/b.git", "", ErrInvalidURL},
		{"github.com/a/b", "", ErrInvalidURL},
		{"https:///a/b", "", ErrInvalidURL},
		{"https://gitlab.com/a/b", "", ErrUnsupportedPlatform},
		{"https://www.github.com/a/b", "", ErrUnsupportedPlatform},
		{"https://github.com.example.org/a/b", "", ErrUnsupportedPlatform},
		{"https://github.com:443/a/b", "", ErrInvalidURL},
		{"https://github.com:/a/b", "", ErrInvalidURL},
		{"https://u:p@github.com/a/b", "", ErrInvalidURL},
		{"https://github.com/a/b?", "", ErrInvalidURL},
		{"https://github.com/a/b#", "", ErrInvalidURL},
		{"https://github.com/a/b?x=1", "", ErrInvalidURL},
		{"https://github.com/a/b#readme", "", ErrInvalidURL},
		{"https://github.com/a/b%2F", "", ErrInvalidURL},
		{"https://github.com/a/b/tree/main", "", ErrInvalidURL},
		{"https://github.com/a/b/blob/main/go.mod", "", ErrInvalidURL},
		{"https://github.com/a/b/pull/1", "", ErrInvalidURL},
		{"https://github.com//b", "", ErrInvalidURL},
		{"https://github.com/a/", "", ErrInvalidURL},
		{"https://github.com/a/b//", "", ErrInvalidURL},
		{"https://github.com/a_b/repo", "", ErrInvalidURL},
		{"https://github.com/a.b/repo", "", ErrInvalidURL},
		{"https://github.com/a/.", "", ErrInvalidURL},
		{"https://github.com/a/..", "", ErrInvalidURL},
		{"https://github.com/a/.git", "", ErrInvalidURL},
		{"https://github.com/a/..git", "", ErrInvalidURL},
		{"https://github.com/a/...git", "", ErrInvalidURL},
		// Error ordering: format before host, host before path restrictions.
		{"http://gitlab.com/a/b", "", ErrInvalidURL},
		{"https://gitlab.com/a/b%2F", "", ErrInvalidURL},
		{"https://gitlab.com:443/a/b?", "", ErrUnsupportedPlatform},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseRepositoryURL(tc.input)
			if !errors.Is(err, tc.failure) {
				t.Fatalf("error = %v, want %v", err, tc.failure)
			}
			if err == nil && got.CanonicalURL() != tc.canonical {
				t.Fatalf("canonical = %q, want %q", got.CanonicalURL(), tc.canonical)
			}
		})
	}
}

func TestRepositoryURLLengthAfterOnlySpaceTrim(t *testing.T) {
	prefix := "https://github.com/a/"
	value := prefix + strings.Repeat("r", 2048-len(prefix))
	if _, err := ParseRepositoryURL("  " + value + "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRepositoryURL(value + "r"); !errors.Is(err, ErrURLTooLong) {
		t.Fatalf("2049 characters: %v", err)
	}
	if _, err := ParseRepositoryURL(strings.Repeat("я", 2048)); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("must count code points, not bytes: %v", err)
	}
	if _, err := ParseRepositoryURL(strings.Repeat("я", 2049)); !errors.Is(err, ErrURLTooLong) {
		t.Fatalf("length must precede character validation: %v", err)
	}
}
