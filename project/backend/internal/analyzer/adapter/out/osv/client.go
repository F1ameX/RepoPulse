// Package osv collects advisory responses for exact dependency versions.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const (
	queryURL         = "https://api.osv.dev/v1/query"
	maxAttempts      = 40
	maxResponseBytes = 5 << 20
	attemptTimeout   = 15 * time.Second
)

var advisoryID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)

type Client struct {
	http *http.Client
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	copy.Jar = nil
	// Redirects must never send an extra request outside the explicit budget.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: &copy}
}

type queryBody struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

// Query uses one budget for the entire dependency set, including all pages
// and retries. A failure keeps previously received pages and results.
func (c *Client) Query(ctx context.Context, queries []domain.OSVQuery) []domain.OSVResult {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	results := make([]domain.OSVResult, 0, len(queries))
	remaining := maxAttempts
	blocked := ""
	for _, query := range queries {
		result := domain.OSVResult{Query: query, ReasonCode: blocked}
		if blocked == "" {
			result = c.query(ctx, query, &remaining)
			switch result.ReasonCode {
			case "OSV_RATE_LIMITED", "OSV_RETRY_DEFERRED":
				// A service-wide backoff also applies to the remaining packages.
				blocked = result.ReasonCode
			}
		}
		results = append(results, result)
	}
	return results
}

func (c *Client) query(ctx context.Context, query domain.OSVQuery, remaining *int) domain.OSVResult {
	result := domain.OSVResult{Query: query}
	switch query.Ecosystem {
	case "npm", "Maven", "Go":
	default:
		result.ReasonCode = "OSV_QUERY_INVALID"
		return result
	}
	if strings.TrimSpace(query.Name) == "" || strings.TrimSpace(query.Version) == "" {
		result.ReasonCode = "OSV_QUERY_INVALID"
		return result
	}
	var request queryBody
	request.Package.Ecosystem, request.Package.Name, request.Version = query.Ecosystem, query.Name, query.Version
	seen := make(map[string]bool)
	for {
		page, next, code := c.fetchPage(ctx, request, remaining)
		if code != "" {
			result.ReasonCode = code
			return result
		}
		result.Pages = append(result.Pages, page)
		if next == "" {
			return result
		}
		if seen[next] {
			result.ReasonCode = "OSV_PAGE_LOOP"
			return result
		}
		seen[next] = true
		request.PageToken = next
	}
}

type failure struct {
	code  string
	retry bool
	delay time.Duration
}

func (c *Client) fetchPage(ctx context.Context, request queryBody, remaining *int) (domain.OSVPage, string, string) {
	body, err := json.Marshal(request)
	if err != nil {
		return domain.OSVPage{}, "", "OSV_QUERY_INVALID"
	}
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return domain.OSVPage{}, "", "OSV_INTERRUPTED"
		}
		if *remaining == 0 {
			return domain.OSVPage{}, "", "OSV_ATTEMPT_LIMIT"
		}
		(*remaining)--
		raw, failed := c.attempt(ctx, body)
		if failed.code == "" {
			page, next, ok := decodePage(raw)
			if !ok {
				return domain.OSVPage{}, "", "OSV_INVALID_RESPONSE"
			}
			return page, next, ""
		}
		if !failed.retry || attempt == 1 {
			return domain.OSVPage{}, "", failed.code
		}
		if *remaining == 0 {
			return domain.OSVPage{}, "", "OSV_ATTEMPT_LIMIT"
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= failed.delay {
			return domain.OSVPage{}, "", failed.code
		}
		timer := time.NewTimer(failed.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return domain.OSVPage{}, "", "OSV_INTERRUPTED"
		case <-timer.C:
		}
	}
}

func (c *Client) attempt(ctx context.Context, body []byte) ([]byte, failure) {
	ctx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(body))
	if err != nil {
		return nil, failure{code: "OSV_QUERY_INVALID"}
	}
	request.GetBody = nil // The caller is responsible for replaying and counting attempts.
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "RepoPulse")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, requestFailure(ctx, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		delay, ok := retryAfter(response.Header)
		if !ok {
			delay = time.Minute
		}
		return nil, failure{"OSV_RATE_LIMITED", true, delay}
	case response.StatusCode >= 500 && response.StatusCode <= 599:
		delay, ok := retryAfter(response.Header)
		if ok {
			return nil, failure{"OSV_RETRY_DEFERRED", true, max(time.Second, delay)}
		}
		return nil, failure{"OSV_UPSTREAM_FAILED", true, time.Second}
	case response.StatusCode != http.StatusOK:
		return nil, failure{code: "OSV_HTTP_ERROR"}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, requestFailure(ctx, err)
	}
	if len(raw) > maxResponseBytes {
		return nil, failure{code: "OSV_RESPONSE_LIMIT"}
	}
	return raw, failure{}
}

func requestFailure(ctx context.Context, err error) failure {
	var networkError net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return failure{"OSV_TIMEOUT", true, time.Second}
	}
	return failure{"OSV_NETWORK_ERROR", true, time.Second}
}

func retryAfter(headers http.Header) (time.Duration, bool) {
	raw := headers.Get("Retry-After")
	if seconds, err := strconv.ParseUint(raw, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(raw); err == nil {
		return max(0, time.Until(at)), true
	}
	return 0, false
}

func decodePage(raw []byte) (domain.OSVPage, string, bool) {
	at := domain.UTC(time.Now())
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return domain.OSVPage{}, "", false
	}
	vulns, hasVulns := fields["vulns"]
	token, hasToken := fields["next_page_token"]
	if len(fields) != 0 && !hasVulns && !hasToken {
		return domain.OSVPage{}, "", false
	}
	var next string
	if hasToken && (bytes.Equal(token, []byte("null")) || json.Unmarshal(token, &next) != nil) {
		return domain.OSVPage{}, "", false
	}
	var entries []struct {
		ID string `json:"id"`
	}
	if hasVulns && (json.Unmarshal(vulns, &entries) != nil || entries == nil) {
		return domain.OSVPage{}, "", false
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !advisoryID.MatchString(entry.ID) {
			return domain.OSVPage{}, "", false
		}
		ids = append(ids, entry.ID)
	}
	return domain.OSVPage{Body: raw, VulnerabilityIDs: ids, ObtainedAt: at}, next, true
}
