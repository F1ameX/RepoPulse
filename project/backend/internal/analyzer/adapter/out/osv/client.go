// Package osv collects advisory responses for exact dependency versions.
package osv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const queryURL = "https://api.osv.dev/v1/query"

var advisoryID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)

type Client struct {
	http *http.Client
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	if copy.Transport == nil {
		copy.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true}
	}
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
	ctx = domain.WithHTTPInputBudget(ctx)
	ctx, cancel := context.WithTimeout(ctx, domain.CoreTimeLimit)
	defer cancel()
	results := make([]domain.OSVResult, 0, len(queries))
	blocked := ""
	for _, query := range queries {
		result := domain.OSVResult{Query: query, ReasonCode: blocked}
		if blocked == "" {
			result = c.query(ctx, query)
			switch result.ReasonCode {
			case "OSV_RATE_LIMITED", "OSV_RETRY_DEFERRED", "OSV_INPUT_LIMIT", "OSV_ATTEMPT_LIMIT":
				// A service-wide backoff also applies to the remaining packages.
				blocked = result.ReasonCode
			}
		}
		results = append(results, result)
	}
	return results
}

func (c *Client) query(ctx context.Context, query domain.OSVQuery) (result domain.OSVResult) {
	result.Query = query
	defer func() { normalizeAdvisories(&result) }()
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
		page, next, code := c.fetchPage(ctx, request)
		if code != "" {
			result.ReasonCode = code
			return result
		}
		result.Pages = append(result.Pages, page)
		if len(result.Pages) == domain.HTTPPageLimit {
			result.ReasonCode = "OSV_PAGE_LIMIT"
			return result
		}
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

func normalizeAdvisories(result *domain.OSVResult) {
	seen := make(map[string][sha256.Size]byte)
	rejected := make(map[string]bool)
	invalid, conflicts := 0, 0
	for i := range result.Pages {
		page := &result.Pages[i]
		var body struct {
			Entries []json.RawMessage `json:"vulns"`
		}
		// The envelope was validated when the page was received.
		if json.Unmarshal(page.Body, &body) != nil {
			invalid++
			continue
		}
		page.VulnerabilityIDs = []string{}
		for _, raw := range body.Entries {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			var advisory map[string]any
			if decoder.Decode(&advisory) != nil {
				invalid++
				continue
			}
			id, ok := advisory["id"].(string)
			if !ok || !advisoryID.MatchString(id) {
				invalid++
				continue
			}
			if rejected[id] {
				continue
			}
			canonical, err := json.Marshal(advisory)
			if err != nil {
				invalid++
				rejected[id] = true
				continue
			}
			fingerprint := sha256.Sum256(canonical)
			if previous, exists := seen[id]; exists {
				if previous != fingerprint {
					rejected[id] = true
					conflicts++
				}
				continue
			}
			seen[id] = fingerprint
			page.VulnerabilityIDs = append(page.VulnerabilityIDs, id)
		}
	}
	if invalid+conflicts == 0 {
		return
	}
	for i := range result.Pages {
		page := &result.Pages[i]
		kept := page.VulnerabilityIDs[:0]
		for _, id := range page.VulnerabilityIDs {
			if !rejected[id] {
				kept = append(kept, id)
			}
		}
		page.VulnerabilityIDs = kept
	}
	result.Excluded = invalid + conflicts
	switch {
	case conflicts > 0 && invalid > 0:
		result.ExcludedReason = "OSV_INVALID_AND_CONFLICTING_RECORDS"
	case conflicts > 0:
		result.ExcludedReason = "OSV_CONFLICTING_RECORD"
	default:
		result.ExcludedReason = "OSV_INVALID_RECORD"
	}
	if result.ReasonCode == "" {
		result.ReasonCode = result.ExcludedReason
	}
}

type failure struct {
	code  string
	retry bool
	delay time.Duration
}

func (c *Client) fetchPage(ctx context.Context, request queryBody) (domain.OSVPage, string, string) {
	body, err := json.Marshal(request)
	if err != nil {
		return domain.OSVPage{}, "", "OSV_QUERY_INVALID"
	}
	for attempt := 1; ; attempt++ {
		raw, failed := c.attempt(ctx, body)
		if failed.code == "" {
			page, next, ok := decodePage(raw)
			if !ok {
				return domain.OSVPage{}, "", "OSV_INVALID_RESPONSE"
			}
			return page, next, ""
		}
		if !failed.retry || attempt == domain.HTTPMaxTries {
			return domain.OSVPage{}, "", failed.code
		}
		if err := domain.HTTPAttemptAvailable(ctx, domain.HTTPOSV); err != nil {
			return domain.OSVPage{}, "", budgetFailure(err).code
		}
		if !domain.HTTPRetryFits(ctx, failed.delay) {
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
	ctx, cancel := context.WithTimeout(ctx, domain.HTTPAttemptTime)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(body))
	if err != nil {
		return nil, failure{code: "OSV_QUERY_INVALID"}
	}
	request.GetBody = nil // The caller is responsible for replaying and counting attempts.
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "RepoPulse")
	if err := domain.TakeHTTPAttempt(ctx, domain.HTTPOSV); err != nil {
		return nil, budgetFailure(err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, requestFailure(ctx, err)
	}
	defer response.Body.Close()
	raw, err := domain.ReadHTTPResponse(ctx, response.Body, response.ContentLength)
	switch {
	case errors.Is(err, domain.ErrHTTPInputLimit):
		return nil, failure{code: "OSV_INPUT_LIMIT"}
	case errors.Is(err, domain.ErrHTTPResponseLimit):
		return nil, failure{code: "OSV_RESPONSE_LIMIT"}
	case err != nil:
		return nil, requestFailure(ctx, err)
	}
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
	return raw, failure{}
}

func budgetFailure(err error) failure {
	switch {
	case errors.Is(err, domain.ErrHTTPInputLimit):
		return failure{code: "OSV_INPUT_LIMIT"}
	case errors.Is(err, domain.ErrHTTPAttemptLimit):
		return failure{code: "OSV_ATTEMPT_LIMIT"}
	default:
		return failure{code: "OSV_INTERRUPTED"}
	}
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
	var entries []json.RawMessage
	if hasVulns && (json.Unmarshal(vulns, &entries) != nil || entries == nil) {
		return domain.OSVPage{}, "", false
	}
	return domain.OSVPage{Body: raw, ObtainedAt: at}, next, true
}
