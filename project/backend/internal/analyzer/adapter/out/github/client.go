// Package github confirms public repository identities using GitHub REST metadata.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const apiOrigin = "https://api.github.com"
const maxResponseBytes = 5 << 20

type Client struct {
	http  *http.Client
	token string
}

func NewClient(client *http.Client, token string) *Client {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	// Every redirect is inspected before making another request. Never send a
	// service credential to a destination supplied by an untrusted Location.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: &copy, token: token}
}

type metadata struct {
	ID            json.Number `json:"id"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	HTMLURL       string      `json:"html_url"`
	Private       *bool       `json:"private"`
	Visibility    string      `json:"visibility"`
	DefaultBranch *string     `json:"default_branch"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (c *Client) Lookup(ctx context.Context, address domain.RepositoryURL) (domain.ValidatedRepository, error) {
	path := "/repos/" + address.Owner + "/" + address.Name
	body, status, headers, err := c.get(ctx, path)
	if err != nil {
		return domain.ValidatedRepository{}, err
	}
	expectedID := ""
	if status == http.StatusMovedPermanently || status == http.StatusTemporaryRedirect {
		// GitHub redirects old names to /repositories/{permanent numeric id}.
		// A redirect to another name alone cannot prove identity continuity.
		location, parseErr := url.Parse(headers.Get("Location"))
		if parseErr != nil {
			return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
		}
		location = (&url.URL{Scheme: "https", Host: "api.github.com", Path: path}).ResolveReference(location)
		expectedID = strings.TrimPrefix(location.Path, "/repositories/")
		if location.Scheme != "https" || location.Host != "api.github.com" || location.User != nil ||
			location.RawQuery != "" || location.ForceQuery || location.Fragment != "" ||
			strings.Contains(headers.Get("Location"), "#") || location.RawPath != "" ||
			!strings.HasPrefix(location.Path, "/repositories/") || !validID(expectedID) {
			return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
		}
		body, status, _, err = c.get(ctx, location.Path)
		if err != nil {
			return domain.ValidatedRepository{}, err
		}
	}
	if status != http.StatusOK {
		return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
	}
	var data metadata
	if json.Unmarshal(body, &data) != nil || !validID(data.ID.String()) || data.Private == nil {
		return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
	}
	if expectedID != "" && data.ID.String() != expectedID {
		return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
	}
	if *data.Private || (data.Visibility != "" && data.Visibility != "public") {
		return domain.ValidatedRepository{}, domain.ErrRepositoryUnavailable
	}
	confirmed, err := domain.ParseRepositoryURL(data.HTMLURL)
	if err != nil || data.Owner.Login == "" || data.Name == "" ||
		data.FullName != data.Owner.Login+"/"+data.Name ||
		!strings.EqualFold(data.Owner.Login, confirmed.Owner) || !strings.EqualFold(data.Name, confirmed.Name) {
		return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
	}
	var movedFrom *string
	if confirmed.CanonicalURL() != address.CanonicalURL() {
		if expectedID == "" {
			return domain.ValidatedRepository{}, domain.ErrRepositoryCheck
		}
		original := address.CanonicalURL()
		movedFrom = &original
	}
	if data.DefaultBranch != nil && *data.DefaultBranch == "" {
		data.DefaultBranch = nil
	}
	return domain.ValidatedRepository{Repository: domain.Repository{
		GitHubID: data.ID.String(), CanonicalURL: confirmed.CanonicalURL(),
		Owner: data.Owner.Login, Name: data.Name, DefaultBranch: data.DefaultBranch,
		// Revision collection belongs to analysis creation, not this preflight.
		CommitSHA: nil,
	}, MovedFrom: movedFrom}, nil
}

func validID(value string) bool {
	if value == "" || value[0] == '0' {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func (c *Client) get(ctx context.Context, path string) ([]byte, int, http.Header, error) {
	for attempt := 0; attempt < 2; attempt++ {
		body, status, headers, err := c.attempt(ctx, path)
		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, 0, nil, domain.ErrUpstreamTimeout
			}
			return nil, 0, nil, ctx.Err()
		}
		delay := time.Second
		retry := false
		switch {
		case err != nil:
			retry = true
		case status == http.StatusTooManyRequests || (status == http.StatusForbidden && rateLimited(headers, body)):
			err = domain.ErrUpstreamRateLimited
			retry, delay = true, rateLimitDelay(headers)
		case status >= 500:
			err, retry = domain.ErrRepositoryCheck, true
		case status == http.StatusNotFound || status == http.StatusUnavailableForLegalReasons:
			err = domain.ErrRepositoryUnavailable
		case status == http.StatusForbidden:
			// A service permission problem must not be reported as a missing repo.
			var failure struct {
				Message string `json:"message"`
			}
			if headers.Get("X-GitHub-SSO") == "" && json.Unmarshal(body, &failure) == nil && (failure.Message == "Forbidden" || failure.Message == "Repository access blocked") {
				err = domain.ErrRepositoryUnavailable
			} else {
				err = domain.ErrRepositoryCheck
			}
		case status != http.StatusOK && status != http.StatusMovedPermanently && status != http.StatusTemporaryRedirect && status != http.StatusConflict:
			err = domain.ErrRepositoryCheck
		}
		if !retry || attempt == 1 {
			return body, status, headers, err
		}
		if headers.Get("Retry-After") != "" {
			delay = max(delay, rateLimitDelay(headers))
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
			return nil, 0, nil, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, 0, nil, domain.ErrUpstreamTimeout
			}
			return nil, 0, nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, 0, nil, domain.ErrRepositoryCheck
}

func (c *Client) attempt(ctx context.Context, path string) ([]byte, int, http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiOrigin+path, nil)
	if err != nil {
		return nil, 0, nil, domain.ErrRepositoryCheck
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "RepoPulse")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		var networkError net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			return nil, 0, nil, domain.ErrUpstreamTimeout
		}
		return nil, 0, nil, domain.ErrRepositoryCheck
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		var networkError net.Error
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			return nil, 0, nil, domain.ErrUpstreamTimeout
		}
		return nil, 0, nil, domain.ErrRepositoryCheck
	}
	if len(body) > maxResponseBytes {
		return nil, 0, nil, domain.ErrRepositoryCheck
	}
	return body, response.StatusCode, response.Header, nil
}

func rateLimited(headers http.Header, body []byte) bool {
	message := strings.ToLower(string(body))
	return headers.Get("X-RateLimit-Remaining") == "0" || headers.Get("Retry-After") != "" ||
		strings.Contains(message, "rate limit") || strings.Contains(message, "abuse detection")
}

func rateLimitDelay(headers http.Header) time.Duration {
	delay := time.Duration(0)
	confirmed := false
	if raw := headers.Get("Retry-After"); raw != "" {
		if seconds, err := strconv.ParseUint(raw, 10, 31); err == nil {
			delay = time.Duration(seconds) * time.Second
			confirmed = true
		} else if date, err := http.ParseTime(raw); err == nil {
			delay = max(0, time.Until(date))
			confirmed = true
		}
	}
	if headers.Get("X-RateLimit-Remaining") == "0" {
		if seconds, err := strconv.ParseInt(headers.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			delay = max(delay, time.Until(time.Unix(seconds, 0)))
			confirmed = true
		}
	}
	if !confirmed {
		// GitHub's minimum for a secondary limit without valid timing headers.
		return time.Minute
	}
	return delay
}
