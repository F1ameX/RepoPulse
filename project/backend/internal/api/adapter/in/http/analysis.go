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

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/analysis"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http/dto"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

type AnalysisService interface {
	CreateAnalysis(context.Context, analysis.CreateAnalysisCommand) (domain.Analysis, error)
	GetAnalysis(context.Context, analysis.GetAnalysisQuery) (domain.Analysis, error)
}

func createAnalysis(service AnalysisService, resolver ContextResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeCreateAnalysis(w, r)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Ожидается JSON с полями request_id и repository_url")
			return
		}
		verified, err := resolveAnalysisContext(r, resolver)
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		if service == nil {
			notImplemented(w, r)
			return
		}
		result, err := service.CreateAnalysis(r.Context(), analysis.CreateAnalysisCommand{
			Context: verified, Request: analysis.CreateAnalysisRequest{RequestID: request.RequestID, RepositoryURL: request.RepositoryURL},
		})
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, dto.CreateAnalysisResponse{
			Outcome: "accepted", AnalysisID: result.ID, ReportID: result.ReportID,
			Status: result.Status, Repository: repositoryDTO(result.Repository),
		})
	}
}

func getAnalysis(service AnalysisService, resolver ContextResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified, err := resolveAnalysisContext(r, resolver)
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		if service == nil {
			notImplemented(w, r)
			return
		}
		result, err := service.GetAnalysis(r.Context(), analysis.GetAnalysisQuery{Context: verified, AnalysisID: r.PathValue("analysis_id")})
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		// A stored failed result is successful HTTP reading, not a transport error.
		response := dto.AnalysisStatusDTO{
			AnalysisID: result.ID, Repository: repositoryDTO(result.Repository), Status: result.Status,
			Stage: result.Stage, CreatedAt: result.CreatedAt.UTC(), StartedAt: result.StartedAt,
			CompletedAt: result.CompletedAt, ReportID: result.ReportID,
		}
		if result.Status == "failed" {
			failure := domain.Failure("ANALYSIS_FAILED", "")
			if result.Error != nil {
				failure = domain.Failure(result.Error.Code, result.Error.TraceID)
			}
			response.Error = &apiError{Code: failure.Code, Message: failure.Message, TraceID: failure.TraceID, FieldErrors: []fieldError{}}
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func repositoryDTO(r domain.RepositoryMetadata) dto.RepositoryDTO {
	return dto.RepositoryDTO{Platform: r.Platform, GitHubID: r.GitHubID, CanonicalURL: r.CanonicalURL,
		Owner: r.Owner, Name: r.Name, DefaultBranch: r.DefaultBranch, CommitSHA: r.CommitSHA}
}

func resolveAnalysisContext(r *http.Request, resolver ContextResolver) (domain.VerifiedContext, error) {
	tokens := sessionTokens(r)
	if tokens.AccountToken == nil && tokens.GuestToken == nil {
		return domain.VerifiedContext{}, domain.ErrSessionRequired
	}
	if resolver == nil {
		return domain.VerifiedContext{}, apidomain.ErrAuthUnavailable
	}
	return resolver.ResolveContext(r.Context(), tokens)
}

// Token decoding rejects case aliases, duplicates, nulls and unknown fields.
func decodeCreateAnalysis(w http.ResponseWriter, r *http.Request) (dto.CreateAnalysisRequest, bool) {
	var result dto.CreateAnalysisRequest
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return result, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || !utf8.Valid(body) {
		return result, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return result, false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || (key != "request_id" && key != "repository_url") || seen[key] {
			return result, false
		}
		var value *string
		if decoder.Decode(&value) != nil || value == nil {
			return result, false
		}
		seen[key] = true
		if key == "request_id" {
			result.RequestID = *value
		} else {
			result.RepositoryURL = *value
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') || len(seen) != 2 {
		return result, false
	}
	_, err = decoder.Token()
	return result, errors.Is(err, io.EOF)
}

func writeAnalysisError(w http.ResponseWriter, r *http.Request, err error) {
	var sessionFailure *apidomain.Error
	if errors.As(err, &sessionFailure) {
		writeSessionError(w, err)
		return
	}
	switch {
	case errors.Is(err, domain.ErrSubmissionChanged):
		writeError(w, http.StatusConflict, "SUBMISSION_CHANGED", "Запрос изменён. Отправьте его заново")
	case errors.Is(err, domain.ErrSubmissionPending):
		writeError(w, http.StatusConflict, "SUBMISSION_PENDING", "Исходная отправка ещё проверяется. Повторите запрос с тем же request_id")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "Анализ недоступен")
	case errors.Is(err, analysis.ErrRequestID):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: apiError{
			Code: "VALIDATION_FAILED", Message: "Исправьте ошибки в полях запроса", TraceID: w.Header().Get("X-Request-ID"),
			FieldErrors: []fieldError{{Field: "request_id", Code: "INVALID_FORMAT", Message: analysis.ErrRequestID.Message}},
		}})
	default:
		var failure *domain.Error
		if errors.As(err, &failure) {
			writeValidationError(w, r, err)
			return
		}
		logger.From(r.Context()).ErrorContext(r.Context(), "Analysis request failed", "code", "INTERNAL_ERROR")
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось выполнить операцию")
	}
}
