package git

import (
	"context"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// HistoryTree reads only object metadata and never checks out a revision.
func (s *Session) HistoryTree(ctx context.Context, sha string) ([]domain.GitFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !hash(sha) {
		return nil, &readError{"GIT_REVISION_UNAVAILABLE"}
	}
	out := &treeOutput{remaining: 100 << 20, pathLimit: s.reader.limits.Paths, withSizes: true}
	err := s.runCommand(ctx, "", out, "ls-tree", "-r", "-z", "-l", "--full-tree", sha)
	if out.limitCode != "" {
		return out.files, &readError{out.limitCode}
	}
	if err != nil || out.invalid || len(out.pending) > 0 {
		return out.files, &readError{"GIT_TREE_INTERRUPTED"}
	}
	return out.files, nil
}
