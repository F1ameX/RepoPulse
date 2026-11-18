package git

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type historyWalk struct {
	ids     map[string]struct{}
	commits []domain.ActivityCommit
	code    string
}

func (s *Session) walkHistory(ctx context.Context, revisionArgs ...string) historyWalk {
	limit := s.reader.limits.Commits
	args := append([]string{"rev-list", "--timestamp", "--max-count=" + strconv.Itoa(limit+1)}, revisionArgs...)
	// Each record has a timestamp, a space, a SHA-1 and a newline.
	output, err := s.command(ctx, "", max(2<<20, int64(limit+1)*64), args...)
	walk := parseHistory(ctx, output, limit)
	if err != nil {
		walk.code = "GIT_HISTORY_INTERRUPTED"
		if ReasonCode(err) == "GIT_OUTPUT_LIMIT" {
			walk.code = "GIT_OUTPUT_LIMIT"
		}
	}
	if ctx.Err() != nil {
		walk.code = "GIT_TIME_LIMIT"
	}
	return walk
}

func parseHistory(ctx context.Context, output []byte, limit int) historyWalk {
	walk := historyWalk{ids: map[string]struct{}{}}
	// rev-list emits committer seconds and SHA only. Date filtering is done
	// after traversal so a skewed parent timestamp cannot hide its ancestors.
	for rest := string(output); rest != "" && ctx.Err() == nil; {
		line, next, terminated := strings.Cut(rest, "\n")
		if !terminated {
			walk.code = "GIT_HISTORY_INVALID"
			break
		}
		rest = next
		fields := strings.Fields(line)
		if len(fields) != 2 || !hash(fields[1]) {
			walk.code = "GIT_HISTORY_INVALID"
			continue
		}
		sha := fields[1]
		seconds, parseErr := strconv.ParseInt(fields[0], 10, 64)
		// Git timestamps are unsigned; the upper bound is 9999-12-31 UTC.
		valid := parseErr == nil && seconds >= 0 && seconds <= 253402300799
		if !valid {
			walk.code = "GIT_HISTORY_INVALID"
		}
		if _, seen := walk.ids[sha]; seen {
			continue
		}
		if len(walk.ids) == limit {
			walk.code = "GIT_COMMIT_LIMIT"
			break
		}
		walk.ids[sha] = struct{}{}
		if valid {
			walk.commits = append(walk.commits, domain.ActivityCommit{SHA: sha, CommittedAt: time.Unix(seconds, 0).UTC()})
		}
	}
	return walk
}

func historySource(scope string, count int, code string) domain.SourceState {
	state := "unavailable"
	if count > 0 {
		state = "partial"
	}
	return source(scope, count, code, state)
}

func (s *Session) readHistory(ctx context.Context, fetchCode string, empty bool) {
	var main historyWalk
	if sha := s.data.Repository.CommitSHA; sha != nil && ctx.Err() == nil {
		// Walk the pinned revision directly, including all parents of merges.
		// The separate all-refs sample can be limited by unrelated branches.
		main = s.walkHistory(ctx, *sha)
		if _, found := main.ids[*sha]; !found && main.code == "" {
			main.code = "GIT_REVISION_UNAVAILABLE"
		}
	} else if !empty {
		main.code = "GIT_REVISION_UNAVAILABLE"
	}
	mainCode := fetchCode
	if mainCode == "" {
		mainCode = main.code
	}
	if ctx.Err() != nil {
		mainCode = "GIT_TIME_LIMIT"
	}
	s.data.MainCommits = main.commits
	s.data.MainWalkComplete = s.data.Repository.CommitSHA != nil && main.code == "" && ctx.Err() == nil
	s.data.Sources = append(s.data.Sources, historySource(domain.ActivityHistoryScope, len(main.commits), mainCode))

	all := main
	if !s.reader.limits.MainHistoryOnly {
		all = s.walkHistory(ctx, "--topo-order", "--all")
	}
	for _, commit := range all.commits {
		item := domain.GitCommit{SHA: commit.SHA, CommittedAt: commit.CommittedAt}
		_, inMain := main.ids[commit.SHA]
		if inMain || mainCode == "" {
			item.Main = &inMain
		}
		s.data.Commits = append(s.data.Commits, item)
	}
	allCode := fetchCode
	if allCode == "" {
		allCode = all.code
	}
	if allCode == "" {
		allCode = mainCode
	}
	if allCode == "" && s.reader.limits.MainHistoryOnly {
		allCode = "GIT_HISTORY_MAIN_ONLY"
	}
	if ctx.Err() != nil {
		allCode = "GIT_TIME_LIMIT"
	}
	s.data.Sources = append(s.data.Sources, historySource("history", len(all.commits), allCode))
}
