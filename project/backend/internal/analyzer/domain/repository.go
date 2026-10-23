// Package domain defines transport-independent Analyzer models and rules.
package domain

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Error contains only a stable public code and a safe user-facing message.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

var (
	ErrURLRequired           = &Error{"URL_REQUIRED", "Укажите URL репозитория"}
	ErrURLTooLong            = &Error{"URL_TOO_LONG", "URL должен содержать не более 2048 символов"}
	ErrInvalidURL            = &Error{"INVALID_REPOSITORY_URL", "Укажите ссылку вида https://github.com/owner/repository"}
	ErrUnsupportedPlatform   = &Error{"UNSUPPORTED_PLATFORM", "Поддерживаются только репозитории GitHub"}
	ErrRepositoryUnavailable = &Error{"REPOSITORY_NOT_FOUND_OR_PRIVATE", "Публичный репозиторий недоступен"}
	ErrRepositoryCheck       = &Error{"REPOSITORY_CHECK_FAILED", "Не удалось проверить доступность репозитория. Повторите попытку позже"}
	ErrUpstreamRateLimited   = &Error{"UPSTREAM_RATE_LIMITED", "Не удалось проверить доступность репозитория. Повторите попытку позже"}
	ErrUpstreamTimeout       = &Error{"UPSTREAM_TIMEOUT", "Не удалось проверить доступность репозитория. Повторите попытку позже"}
)

type RepositoryURL struct {
	Owner string
	Name  string
}

func (r RepositoryURL) CanonicalURL() string {
	return "https://github.com/" + strings.ToLower(r.Owner) + "/" + strings.ToLower(r.Name)
}

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
var namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ParseRepositoryURL implements FR-01 in its specified error order. Only outer
// U+0020 spaces, one trailing slash and one literal .git suffix are removed.
func ParseRepositoryURL(raw string) (RepositoryURL, error) {
	value := strings.Trim(raw, " ")
	if value == "" {
		return RepositoryURL{}, ErrURLRequired
	}
	if utf8.RuneCountInString(value) > 2048 {
		return RepositoryURL{}, ErrURLTooLong
	}
	for _, ch := range value {
		if ch <= 0x20 || ch >= 0x7f || ch == '\\' || ch == '%' {
			return RepositoryURL{}, ErrInvalidURL
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.Opaque != "" {
		return RepositoryURL{}, ErrInvalidURL
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return RepositoryURL{}, ErrUnsupportedPlatform
	}
	if parsed.User != nil || !strings.EqualFold(parsed.Host, "github.com") || strings.ContainsAny(value, "?#") {
		return RepositoryURL{}, ErrInvalidURL
	}
	parts := strings.Split(strings.TrimSuffix(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "" || !ownerPattern.MatchString(parts[1]) || !namePattern.MatchString(parts[2]) {
		return RepositoryURL{}, ErrInvalidURL
	}
	name := strings.TrimSuffix(parts[2], ".git")
	if name == "" || name == "." || name == ".." {
		return RepositoryURL{}, ErrInvalidURL
	}
	return RepositoryURL{Owner: strings.ToLower(parts[1]), Name: strings.ToLower(name)}, nil
}

// Repository is backed by confirmed GitHub metadata. Unknown revisions remain nil.
type Repository struct {
	GitHubID      string
	CanonicalURL  string
	Owner         string
	Name          string
	DefaultBranch *string
	CommitSHA     *string
}

type ValidatedRepository struct {
	Repository Repository
	MovedFrom  *string
}
