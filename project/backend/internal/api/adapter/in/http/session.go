package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http/dto"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

type Sessions interface {
	GetSessionContext(context.Context, domain.SessionTokens) (domain.SessionContext, error)
	ContinueGuest(context.Context, domain.SessionTokens) (domain.IssuedSession, bool, error)
}

type Options struct {
	Sessions     Sessions
	CookieSecure bool
	PublicOrigin string
}

func sessionTokens(r *http.Request) domain.SessionTokens {
	tokens := domain.SessionTokens{}
	if cookie, err := r.Cookie("repopulse_session"); err == nil {
		tokens.AccountToken = &cookie.Value
	}
	if cookie, err := r.Cookie("repopulse_guest"); err == nil {
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
		http.SetCookie(w, &http.Cookie{Name: "repopulse_guest", Value: issued.Token,
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
	switch failure.Code {
	case "SESSION_REQUIRED", "SESSION_EXPIRED":
		status = http.StatusUnauthorized
	case "AUTH_UNAVAILABLE":
		status = http.StatusServiceUnavailable
	case "NOT_IMPLEMENTED":
		status = http.StatusNotImplemented
	}
	writeError(w, status, failure.Code, failure.Message)
}
