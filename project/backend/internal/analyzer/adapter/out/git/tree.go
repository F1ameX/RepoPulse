package git

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// treeOutput keeps complete NUL-terminated records as Git emits them, even
// when traversal is later interrupted. It stops Git once either budget is hit.
type treeOutput struct {
	remaining int
	pathLimit int
	withSizes bool
	paths     int
	pending   []byte
	files     []domain.GitFile
	invalid   bool
	limitCode string
}

func (out *treeOutput) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		if out.paths == out.pathLimit {
			out.limitCode = "GIT_PATH_LIMIT"
			return n, &readError{out.limitCode}
		}
		if out.remaining == 0 {
			out.limitCode = "GIT_OUTPUT_LIMIT"
			return n, &readError{out.limitCode}
		}
		chunk := p[:min(len(p), out.remaining)]
		end := bytes.IndexByte(chunk, 0)
		take := len(chunk)
		if end >= 0 {
			take = end + 1
			chunk = chunk[:end]
		}
		out.pending = append(out.pending, chunk...)
		out.remaining -= take
		n += take
		p = p[take:]
		if end >= 0 {
			out.paths++
			if file, ok := parseTreeEntry(out.pending, out.withSizes); ok {
				out.files = append(out.files, file)
			} else {
				out.invalid = true
			}
			out.pending = out.pending[:0]
		}
	}
	return n, nil
}

func parseTreeEntry(record []byte, withSizes bool) (domain.GitFile, bool) {
	header, path, ok := bytes.Cut(record, []byte{'\t'})
	fields := strings.Fields(string(header))
	wantFields := 3
	if withSizes {
		wantFields = 4
	}
	if !ok || len(path) == 0 || !utf8.Valid(path) || len(fields) != wantFields || !hash(fields[2]) {
		return domain.GitFile{}, false
	}
	size := int64(-1)
	switch fields[0] {
	case "100644", "100755", "120000":
		if fields[1] != "blob" {
			return domain.GitFile{}, false
		}
		if withSizes {
			var err error
			size, err = strconv.ParseInt(fields[3], 10, 64)
			if err != nil || size < 0 {
				return domain.GitFile{}, false
			}
		}
	case "160000":
		if fields[1] != "commit" || withSizes && fields[3] != "-" {
			return domain.GitFile{}, false
		}
	default:
		return domain.GitFile{}, false
	}
	return domain.GitFile{Path: string(path), Mode: fields[0], ObjectSHA: fields[2], Size: size}, true
}

// The pinned tree can establish file paths and modes without history or blobs.
// Use a separate bare repository so the shallow boundary cannot affect history.
func (s *Session) collectPinnedTree(ctx context.Context, remote string) ([]domain.GitFile, domain.SourceState) {
	tree := &Session{
		reader: s.reader, root: s.root, dir: filepath.Join(s.root, "pinned-tree.git"),
		data: domain.GitData{Repository: s.data.Repository},
	}
	if _, err := tree.command(ctx, "", 1<<20, "init", "--bare", "--template="+filepath.Join(s.root, "hooks"), tree.dir); err != nil {
		return nil, domain.SourceState{}
	}
	pinned := "+" + *s.data.Repository.CommitSHA + ":refs/repopulse/pinned"
	if _, err := tree.command(ctx, "", 1<<20, "fetch", "--depth=1", "--filter=blob:none", "--no-tags",
		"--no-recurse-submodules", "--no-auto-maintenance", "--no-write-fetch-head", remote, pinned); err != nil {
		return nil, domain.SourceState{}
	}
	tree.readTree(ctx, "", false, false)
	return tree.data.Files, tree.data.Sources[0]
}

func (s *Session) readTree(ctx context.Context, fetchCode string, empty, withSizes bool) {
	code := ""
	sha := s.data.Repository.CommitSHA
	if sha == nil {
		code = fetchCode
		if code == "" && !empty {
			code = "GIT_REVISION_UNAVAILABLE"
		}
	} else {
		kind, err := s.command(ctx, "", 64, "cat-file", "-t", *sha)
		if err != nil || strings.TrimSpace(string(kind)) != "commit" {
			code = fetchCode
			if code == "" {
				code = "GIT_REVISION_UNAVAILABLE"
			}
		} else {
			// Read every leaf path at the pinned commit, including nested files.
			// Neither a checkout nor a language/extension filter is needed.
			out := &treeOutput{remaining: 100 << 20, pathLimit: s.reader.limits.Paths, withSizes: withSizes}
			args := []string{"ls-tree", "-r", "-z", "--full-tree"}
			if withSizes {
				args = append(args, "-l")
			}
			err := s.runCommand(ctx, "", out, append(args, *sha)...)
			s.data.Files = append(s.data.Files, out.files...)
			switch {
			case out.limitCode != "":
				code = out.limitCode
			case err != nil:
				code = "GIT_TREE_INTERRUPTED"
			case out.invalid || len(out.pending) > 0:
				code = "GIT_TREE_INVALID"
			}
		}
	}
	if ctx.Err() != nil {
		code = "GIT_TIME_LIMIT"
	}
	state := "unavailable"
	if len(s.data.Files) > 0 {
		state = "partial"
	}
	s.data.Sources = append(s.data.Sources, source(domain.GitTreeScope, len(s.data.Files), code, state))
}
