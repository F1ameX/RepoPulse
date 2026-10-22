package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

type RepositoryValidator interface {
	ValidateRepository(context.Context, repository.ValidateRepositoryCommand) (domain.ValidatedRepository, error)
}

type SessionTokens struct {
	AccountToken *string
	GuestToken   *string
}

// ContextResolver is implemented by an Auth adapter. Cookie presence alone does
// not establish an identity; only Auth may produce a VerifiedContext.
type ContextResolver interface {
	ResolveContext(context.Context, SessionTokens) (domain.VerifiedContext, error)
}

type repositoryDTO struct {
	Platform      string  `json:"platform"`
	GitHubID      string  `json:"github_id"`
	CanonicalURL  string  `json:"canonical_url"`
	Owner         string  `json:"owner"`
	Name          string  `json:"name"`
	DefaultBranch *string `json:"default_branch"`
	CommitSHA     *string `json:"commit_sha"`
}

type validatedRepositoryDTO struct {
	Repository repositoryDTO `json:"repository"`
	MovedFrom  *string       `json:"moved_from"`
}

func validateRepository(validator RepositoryValidator, resolver ContextResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Ожидается JSON с полем repository_url")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Тело запроса превышает 1 MiB")
			} else {
				writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Не удалось прочитать JSON запроса")
			}
			return
		}
		rawURL, ok := decodeRepositoryURL(body)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Ожидается JSON с единственным строковым полем repository_url")
			return
		}
		tokens := SessionTokens{}
		if cookie, err := r.Cookie("repopulse_session"); err == nil {
			tokens.AccountToken = &cookie.Value
		}
		if cookie, err := r.Cookie("repopulse_guest"); err == nil {
			tokens.GuestToken = &cookie.Value
		}
		if tokens.AccountToken == nil && tokens.GuestToken == nil {
			writeValidationError(w, r, domain.ErrSessionRequired)
			return
		}
		if resolver == nil {
			writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Проверка сессии через Auth ещё не подключена")
			return
		}
		verifiedContext, err := resolver.ResolveContext(r.Context(), tokens)
		if err != nil {
			writeValidationError(w, r, err)
			return
		}
		if validator == nil {
			notImplemented(w, r)
			return
		}
		result, err := validator.ValidateRepository(r.Context(), repository.ValidateRepositoryCommand{
			Context: verifiedContext,
			Request: repository.ValidateRepositoryRequest{RepositoryURL: rawURL},
		})
		if err != nil {
			writeValidationError(w, r, err)
			return
		}
		repository := result.Repository
		writeJSON(w, http.StatusOK, validatedRepositoryDTO{Repository: repositoryDTO{
			Platform: "github", GitHubID: repository.GitHubID, CanonicalURL: repository.CanonicalURL,
			Owner: repository.Owner, Name: repository.Name, DefaultBranch: repository.DefaultBranch,
			CommitSHA: repository.CommitSHA,
		}, MovedFrom: result.MovedFrom})
	}
}

// Decode tokens explicitly: encoding/json's struct matching would also accept
// Repository_Url and duplicate keys, contrary to the strict JSON contract.
func decodeRepositoryURL(body []byte) (string, bool) {
	if !utf8.Valid(body) {
		return "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return "", false
	}
	var value *string
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil || key != "repository_url" || seen {
			return "", false
		}
		seen = true
		if err := decoder.Decode(&value); err != nil || value == nil {
			return "", false
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') || !seen {
		return "", false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", false
	}
	return *value, true
}

func writeValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var failure *domain.Error
	if !errors.As(err, &failure) {
		logger.From(r.Context()).ErrorContext(r.Context(), "Repository validation failed", "code", "INTERNAL_ERROR")
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось выполнить операцию")
		return
	}
	status := http.StatusServiceUnavailable
	fields := []fieldError{}
	switch failure.Code {
	case "SESSION_REQUIRED", "SESSION_EXPIRED":
		status = http.StatusUnauthorized
	case "URL_REQUIRED", "URL_TOO_LONG", "INVALID_REPOSITORY_URL", "UNSUPPORTED_PLATFORM":
		status = http.StatusBadRequest
		fields = append(fields, fieldError{Field: "repository_url", Code: failure.Code, Message: failure.Message})
	case "REPOSITORY_NOT_FOUND_OR_PRIVATE":
		status = http.StatusNotFound
	case "UPSTREAM_TIMEOUT":
		status = http.StatusGatewayTimeout
	}
	writeJSON(w, status, errorResponse{Error: apiError{
		Code: failure.Code, Message: failure.Message, TraceID: w.Header().Get("X-Request-ID"), FieldErrors: fields,
	}})
}
