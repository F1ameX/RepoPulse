package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http/dto"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

type Sessions interface {
	GetSessionContext(context.Context, domain.SessionTokens) (domain.SessionContext, error)
	ContinueGuest(context.Context, domain.SessionTokens) (domain.IssuedSession, bool, error)
	Register(ctx context.Context, login, password string) (domain.RegistrationResult, error)
	Login(ctx context.Context, login, password string) (domain.IssuedSession, error)
	Logout(context.Context, domain.SessionTokens) error
}

const (
	accountCookie = "repopulse_session"
	guestCookie   = "repopulse_guest"
)

type Options struct {
	Sessions     Sessions
	CookieSecure bool
	PublicOrigin string
}

func sessionTokens(r *http.Request) domain.SessionTokens {
	tokens := domain.SessionTokens{}
	if cookie, err := r.Cookie(accountCookie); err == nil {
		tokens.AccountToken = &cookie.Value
	}
	if cookie, err := r.Cookie(guestCookie); err == nil {
		tokens.GuestToken = &cookie.Value
	}
	return tokens
}

func getSession(sessions Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessions == nil {
			notImplemented(w, r)
			return
		}
		result, err := sessions.GetSessionContext(r.Context(), sessionTokens(r))
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, dto.SessionContext{Account: dto.Session(result.Account), Guest: dto.Session(result.Guest)})
	}
}

func continueGuest(sessions Sessions, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessions == nil {
			notImplemented(w, r)
			return
		}
		issued, created, err := sessions.ContinueGuest(r.Context(), sessionTokens(r))
		if err != nil {
			writeSessionError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: guestCookie, Value: issued.Token,
			Path: "/api/v1", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
		code := http.StatusOK
		if created {
			code = http.StatusCreated
		}
		writeJSON(w, code, dto.Session(&issued.Session))
	}
}

// The configured browser origin is exact; missing/null/cross-origin requests
// cannot create a cookie or use the cookie-authenticated repository check.
func sameOrigin(origin string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected, err := url.Parse(origin)
		actual, actualErr := url.Parse(r.Header.Get("Origin"))
		if err != nil || actualErr != nil || actual.Scheme == "" || actual.Host == "" ||
			actual.Scheme != expected.Scheme || actual.Host != expected.Host || actual.User != nil ||
			actual.Path != "" || actual.RawQuery != "" || actual.Fragment != "" || actual.ForceQuery ||
			r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeError(w, http.StatusForbidden, "CSRF_REJECTED", "Запрос не прошёл проверку источника")
			return
		}
		next(w, r)
	}
}

func writeSessionError(w http.ResponseWriter, err error) {
	failure := domain.ErrInternal
	var known *domain.Error
	if errors.As(err, &known) {
		failure = known
	}
	status := http.StatusInternalServerError
	fields := []fieldError{}
	switch failure.Code {
	case "VALIDATION_FAILED":
		status = http.StatusBadRequest
		for _, f := range failure.FieldErrors {
			fields = append(fields, fieldError{Field: f.Field, Code: f.Code, Message: f.Message})
		}
	case "SESSION_REQUIRED", "SESSION_EXPIRED", "INVALID_CREDENTIALS":
		status = http.StatusUnauthorized
	case "LOGIN_TAKEN":
		status = http.StatusConflict
	case "AUTH_UNAVAILABLE":
		status = http.StatusServiceUnavailable
	case "NOT_IMPLEMENTED":
		status = http.StatusNotImplemented
	}
	writeJSON(w, status, errorResponse{Error: apiError{
		Code: failure.Code, Message: failure.Message, TraceID: w.Header().Get("X-Request-ID"), FieldErrors: fields,
	}})
}

// register creates an account (201). No cookie is set: registration does not sign in.
func register(sessions Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessions == nil {
			notImplemented(w, r)
			return
		}
		login, password, ok := readCredentials(w, r)
		if !ok {
			return
		}
		result, err := sessions.Register(r.Context(), login, password)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, dto.RegistrationResult{Login: result.Login, Message: result.Message})
	}
}

// login issues a persistent account cookie that lives exactly until the session
// expires; the guest cookie is left untouched (FR-27).
func login(sessions Sessions, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessions == nil {
			notImplemented(w, r)
			return
		}
		login, password, ok := readCredentials(w, r)
		if !ok {
			return
		}
		issued, err := sessions.Login(r.Context(), login, password)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		// The lifetime is configurable in Auth, so Max-Age follows expires_at instead of a constant.
		maxAge := int(time.Until(issued.Session.ExpiresAt).Seconds())
		if maxAge <= 0 {
			writeSessionError(w, domain.ErrInternal)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: accountCookie, Value: issued.Token, Path: "/api/v1",
			MaxAge: maxAge, Expires: issued.Session.ExpiresAt.UTC(),
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
		writeJSON(w, http.StatusOK, dto.Session(&issued.Session))
	}
}

// logout revokes the current account session and deletes the account cookie (204).
// It is idempotent. If Auth cannot confirm the revocation, the error is returned
// and the cookie is kept, so the client can retry instead of assuming success.
// The guest cookie is never deleted.
func logout(sessions Sessions, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessions == nil {
			notImplemented(w, r)
			return
		}
		if err := sessions.Logout(r.Context(), sessionTokens(r)); err != nil {
			writeSessionError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: accountCookie, Value: "", Path: "/api/v1", MaxAge: -1,
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(http.StatusNoContent)
	}
}

// readCredentials reads a JSON object with exactly the string fields login and
// password. On failure it writes 400 and returns false. Values are not logged.
func readCredentials(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	const message = "Ожидается JSON с полями login и password"
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", message)
		return "", "", false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", message)
		return "", "", false
	}
	login, password, ok := decodeCredentials(body)
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", message)
		return "", "", false
	}
	return login, password, true
}

// decodeCredentials accepts only {"login": string, "password": string}: no
// duplicate, unknown or differently cased keys, no null and no trailing data.
func decodeCredentials(body []byte) (string, string, bool) {
	if !utf8.Valid(body) {
		return "", "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return "", "", false
	}
	values := map[string]*string{}
	for decoder.More() {
		key, err := decoder.Token()
		name, isString := key.(string)
		if err != nil || !isString || (name != "login" && name != "password") || values[name] != nil {
			return "", "", false
		}
		var value *string
		if err := decoder.Decode(&value); err != nil || value == nil {
			return "", "", false
		}
		values[name] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return "", "", false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", "", false
	}
	if values["login"] == nil || values["password"] == nil {
		return "", "", false
	}
	return *values["login"], *values["password"], true
}
