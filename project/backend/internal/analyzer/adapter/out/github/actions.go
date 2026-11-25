package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const actionsSearchLimit = 1000

type actionsPage struct {
	Total *int              `json:"total_count"`
	Runs  []json.RawMessage `json:"workflow_runs"`
}

type actionsRecord struct {
	ID         json.Number `json:"id"`
	Attempt    int         `json:"run_attempt"`
	HeadSHA    string      `json:"head_sha"`
	HeadBranch string      `json:"head_branch"`
	Status     string      `json:"status"`
	Conclusion string      `json:"conclusion"`
	CreatedAt  time.Time   `json:"created_at"`
	StartedAt  *time.Time  `json:"run_started_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

func decodeAttempt(raw []byte, at time.Time, branch string) (domain.ActionsAttempt, bool) {
	var item actionsRecord
	if json.Unmarshal(raw, &item) != nil || item.HeadBranch != branch {
		return domain.ActionsAttempt{}, false
	}
	if item.StartedAt != nil {
		utc := domain.UTC(*item.StartedAt)
		item.StartedAt = &utc
	}
	result := domain.ActionsAttempt{
		RunID: item.ID.String(), Attempt: item.Attempt, HeadSHA: item.HeadSHA,
		Status: item.Status, Conclusion: item.Conclusion, CreatedAt: domain.UTC(item.CreatedAt),
		StartedAt: item.StartedAt, UpdatedAt: domain.UTC(item.UpdatedAt), ObtainedAt: at,
	}
	// The REST API supplies numeric IDs; json.Number also accepts quoted values,
	// but in either form only a positive integer identifier is retained.
	_, idErr := strconv.ParseUint(result.RunID, 10, 64)
	return result, idErr == nil && result.Valid()
}

type attemptKey struct {
	id      string
	attempt int
}

func readAttemptKey(raw []byte) attemptKey {
	var id struct {
		Run     json.Number `json:"id"`
		Attempt int         `json:"run_attempt"`
	}
	if json.Unmarshal(raw, &id) != nil || !validID(id.Run.String()) || id.Attempt < 1 {
		return attemptKey{}
	}
	return attemptKey{id.Run.String(), id.Attempt}
}

type actionsSample struct {
	items     map[attemptKey]domain.ActionsAttempt
	rejected  map[attemptKey]bool
	invalid   int
	conflicts int
}

func (s *actionsSample) exclude(key attemptKey) {
	if key.id == "" {
		s.invalid++
	} else if !s.rejected[key] {
		s.invalid++
		delete(s.items, key)
		s.rejected[key] = true
	}
}

func (s *actionsSample) add(item domain.ActionsAttempt) {
	key := attemptKey{item.RunID, item.Attempt}
	if s.rejected[key] {
		return
	}
	if previous, found := s.items[key]; found {
		sameStart := previous.StartedAt == nil && item.StartedAt == nil ||
			previous.StartedAt != nil && item.StartedAt != nil && previous.StartedAt.Equal(*item.StartedAt)
		previous.StartedAt, previous.ObtainedAt = item.StartedAt, item.ObtainedAt
		if !sameStart || previous != item {
			delete(s.items, key)
			s.rejected[key] = true
			s.conflicts++
			return
		}
	}
	s.items[key] = item
}

func (b *ciBudget) collectActions(ctx context.Context, root string, repository domain.RepositoryMetadata, window domain.Window, ci *domain.GitHubCI) domain.SourceState {
	ctx = domain.WithHTTPInputBudget(ctx)
	if repository.DefaultBranch == nil {
		return ciSource(domain.ActionsScope, "GITHUB_CI_BRANCH_UNAVAILABLE", time.Time{}, 0, 0, 0)
	}
	branch := *repository.DefaultBranch
	path := root + "/actions/runs"
	record := func(key attemptKey) bool {
		return key.id == "" || domain.RecordGitHubObject(ctx, fmt.Sprintf("%s/%s/attempts/%d", path, key.id, key.attempt))
	}
	filters := url.Values{
		"branch":  {branch},
		"created": {window.From.UTC().Format(time.RFC3339) + ".." + window.Until.UTC().Format(time.RFC3339)},
	}
	sample := actionsSample{items: map[attemptKey]domain.ActionsAttempt{}, rejected: map[attemptKey]bool{}}
	totals := make(map[string]domain.ActionsAttempt)
	listed := make(map[string]bool)
	announced, code, at := -1, "", time.Time{}
	for page := 1; ; page++ {
		response, failed := b.get(ctx, pagePath(path, filters, page))
		if failed != "" {
			code = failed
			break
		}
		var payload actionsPage
		if json.Unmarshal(response.body, &payload) != nil || payload.Total == nil || *payload.Total < 0 || payload.Runs == nil ||
			len(payload.Runs) > ciPageSize || len(payload.Runs) > *payload.Total {
			code = "GITHUB_CI_INVALID_RESPONSE"
			break
		}
		at = response.obtainedAt
		if announced == -1 {
			announced = *payload.Total
		} else if announced != *payload.Total {
			code = "GITHUB_CI_INCONSISTENT_PAGES"
		}
		overflow := false
		for _, raw := range payload.Runs {
			key := readAttemptKey(raw)
			if !record(key) {
				overflow = true
				break
			}
			item, valid := decodeAttempt(raw, at, branch)
			if !valid {
				sample.exclude(key)
				continue
			}
			listed[item.RunID] = true
			// Check duplicates even when a conflicting date puts one copy
			// outside the requested window. Filter only after deduplication.
			sample.add(item)
			if item.CreatedAt.Before(window.From) || !item.CreatedAt.Before(window.Until) {
				continue
			}
			if previous := totals[item.RunID]; item.Attempt > previous.Attempt {
				totals[item.RunID] = item
			}
		}
		next, valid := nextCIPage(response.headers, path, repository.GitHubID, page)
		switch {
		case overflow || next && domain.GitHubObjectsExhausted(ctx):
			code = "GITHUB_CI_OBJECT_LIMIT"
			next = false
		case !valid:
			code = "GITHUB_CI_INVALID_PAGINATION"
		case page == domain.HTTPPageLimit:
			code = "GITHUB_CI_PAGE_LIMIT"
			next = false
		case !next && announced >= actionsSearchLimit:
			code = "GITHUB_CI_SEARCH_LIMIT"
			next = false
		case !next && announced != len(listed) && code == "" && sample.invalid == 0:
			code = "GITHUB_CI_INCONSISTENT_PAGES"
		}
		if !next || !valid {
			break
		}
	}
	runIDs := make([]string, 0, len(totals))
	for id := range totals {
		runIDs = append(runIDs, id)
	}
	sort.Strings(runIDs)
	// GitHub lists the current attempt; recover missing earlier attempts using
	// the per-attempt endpoint, without repeating attempts already on the pages.
backfill:
	for _, id := range runIDs {
		latest := totals[id]
		for number := latest.Attempt - 1; number > 0; number-- {
			key := attemptKey{id, number}
			if _, seen := sample.items[key]; seen || sample.rejected[key] {
				continue
			}
			response, failed := b.get(ctx, fmt.Sprintf("%s/%s/attempts/%d", path, id, number))
			if failed != "" {
				if code == "" {
					code = failed
				}
				if failed == "GITHUB_CI_OBJECT_LIMIT" || failed == "GITHUB_CI_INPUT_LIMIT" || failed == "GITHUB_CI_REQUEST_LIMIT" ||
					ctx.Err() != nil || b.blocked != "" || b.remaining == 0 {
					break backfill
				}
				continue
			}
			if !record(key) {
				code = "GITHUB_CI_OBJECT_LIMIT"
				break backfill
			}
			item, valid := decodeAttempt(response.body, response.obtainedAt, branch)
			if !valid || item.RunID != id || item.Attempt != number || item.HeadSHA != latest.HeadSHA {
				sample.exclude(key)
				continue
			}
			at = response.obtainedAt
			sample.add(item)
		}
	}
	for _, item := range sample.items {
		// A rerun can become visible after assessment time; its predecessor
		// still belongs to the accepted analysis window.
		if item.CreatedAt.Before(window.From) || !item.CreatedAt.Before(window.Until) ||
			item.StartedAt != nil && !item.StartedAt.Before(window.Until) {
			continue
		}
		if !item.UpdatedAt.Before(window.Until) && code == "" {
			code = "GITHUB_CI_STATE_CHANGED_AFTER_WINDOW"
		}
		ci.Attempts = append(ci.Attempts, item)
	}
	sort.Slice(ci.Attempts, func(i, j int) bool {
		a, b := ci.Attempts[i], ci.Attempts[j]
		return a.RunID < b.RunID || a.RunID == b.RunID && a.Attempt < b.Attempt
	})
	return ciSource(domain.ActionsScope, code, at, len(ci.Attempts), sample.invalid, sample.conflicts)
}
