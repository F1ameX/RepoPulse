package git

import (
	"bytes"
	"context"
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
			if file, ok := parseTreeEntry(out.pending); ok {
				out.files = append(out.files, file)
			} else {
				out.invalid = true
			}
			out.pending = out.pending[:0]
		}
	}
	return n, nil
}

func parseTreeEntry(record []byte) (domain.GitFile, bool) {
	header, path, ok := bytes.Cut(record, []byte{'\t'})
	fields := strings.Fields(string(header))
	if !ok || len(path) == 0 || !utf8.Valid(path) || len(fields) != 4 || !hash(fields[2]) {
		return domain.GitFile{}, false
	}
	var size int64
	switch fields[0] {
	case "100644", "100755", "120000":
		var err error
		size, err = strconv.ParseInt(fields[3], 10, 64)
		if fields[1] != "blob" || err != nil || size < 0 {
			return domain.GitFile{}, false
		}
	case "160000":
		if fields[1] != "commit" || fields[3] != "-" {
			return domain.GitFile{}, false
		}
		size = -1
	default:
		return domain.GitFile{}, false
	}
	return domain.GitFile{Path: string(path), Mode: fields[0], ObjectSHA: fields[2], Size: size}, true
}

func (s *Session) readTree(ctx context.Context, fetchCode string, empty bool) {
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
			out := &treeOutput{remaining: 100 << 20, pathLimit: s.reader.limits.Paths}
			err := s.runCommand(ctx, "", out, "ls-tree", "-r", "-l", "-z", "--full-tree", *sha)
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
