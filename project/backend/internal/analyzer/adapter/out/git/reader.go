// Package git reads public GitHub objects without checkout or execution of any
// repository code, hooks, filters, submodules, package managers or tests.
package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type Limits struct {
	Timeout        time.Duration
	Bytes          int64
	Paths, Commits int
}

func DefaultLimits() Limits {
	return Limits{Timeout: 60 * time.Second, Bytes: 100 << 20, Paths: 50000, Commits: 10000}
}

type Reader struct {
	binary, tempRoot string
	limits           Limits
	client           *http.Client
}

func NewReader(tempRoot string, limits Limits) (*Reader, error) {
	max := DefaultLimits()
	if limits.Timeout <= 0 || limits.Timeout > max.Timeout || limits.Bytes <= 0 || limits.Bytes > max.Bytes || limits.Paths <= 0 || limits.Paths > max.Paths || limits.Commits <= 0 || limits.Commits > max.Commits {
		return nil, errors.New("invalid Git limits")
	}
	binary, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("Git executable is required")
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DisableCompression: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Reader{binary: binary, tempRoot: tempRoot, limits: limits, client: client}, nil
}

type Session struct {
	mu        sync.Mutex
	reader    *Reader
	root, dir string
	data      domain.GitData
	closed    bool
	blobBytes int64
	ctx       context.Context
}

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func (s *Session) Data() domain.GitData {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.data
	data.Repository.CommitSHA = clonePointer(data.Repository.CommitSHA)
	data.Repository.DefaultBranch = clonePointer(data.Repository.DefaultBranch)
	data.Refs = append([]domain.GitRef{}, data.Refs...)
	data.Files = append([]domain.GitFile{}, data.Files...)
	data.Commits = append([]domain.GitCommit{}, data.Commits...)
	for i := range data.Commits {
		data.Commits[i].Main = clonePointer(data.Commits[i].Main)
	}
	data.Sources = append([]domain.SourceState{}, data.Sources...)
	for i := range data.Sources {
		source := &data.Sources[i]
		source.Reason = clonePointer(source.Reason)
		source.ObtainedAt = clonePointer(source.ObtainedAt)
		source.Fingerprint = clonePointer(source.Fingerprint)
		sample := &source.Sample
		sample.Collected = clonePointer(sample.Collected)
		sample.Total = clonePointer(sample.Total)
		sample.Numerator = clonePointer(sample.Numerator)
		sample.Denominator = clonePointer(sample.Denominator)
		sample.Excluded = clonePointer(sample.Excluded)
		sample.ExcludedReason = clonePointer(sample.ExcludedReason)
	}
	return data
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	err := os.RemoveAll(s.root)
	if err == nil {
		s.closed = true
	}
	return err
}

type readError struct{ code string }

func (e *readError) Error() string { return e.code }
func ReasonCode(err error) string {
	var failure *readError
	if errors.As(err, &failure) {
		return failure.code
	}
	return "GIT_READ_FAILED"
}

// command receives only explicit arguments and a clean environment. No user Git
// configuration, credential helper, pager, textconv or shell command is inherited.
func (s *Session) command(ctx context.Context, input string, limit int64, args ...string) ([]byte, error) {
	prefix := []string{"--no-pager", "--git-dir=" + s.dir, "-c", "core.hooksPath=" + filepath.Join(s.root, "hooks"), "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-c", "credential.interactive=false", "-c", "protocol.allow=never", "-c", "protocol.http.allow=always", "-c", "http.followRedirects=false", "-c", "gc.auto=0", "-c", "fetch.unpackLimit=1", "-c", "fetch.fsckObjects=true", "-c", "submodule.recurse=false"}
	cmd := exec.CommandContext(ctx, s.reader.binary, append(prefix, args...)...)
	cmd.Dir = s.root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=" + filepath.Join(s.root, "disabled-askpass"), "GIT_LFS_SKIP_SMUDGE=1"}
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 250 * time.Millisecond
	isolateProcess(cmd)
	output := &limitedBuffer{max: limit}
	cmd.Stdout = output
	err := cmd.Run()
	if ctx.Err() != nil {
		return output.Bytes(), &readError{"GIT_TIME_LIMIT"}
	}
	if output.exceeded {
		return output.Bytes(), &readError{"GIT_OUTPUT_LIMIT"}
	}
	if err != nil {
		return output.Bytes(), &readError{"GIT_READ_FAILED"}
	}
	return output.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	max      int64
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	left := b.max - int64(b.Len())
	if int64(len(p)) > left {
		if left > 0 {
			_, _ = b.Buffer.Write(p[:int(left)])
		}
		b.exceeded = true
		return 0, errors.New("Git output budget")
	}
	return b.Buffer.Write(p)
}

func (r *Reader) Open(ctx context.Context, metadata domain.RepositoryMetadata) (worker.GitSession, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(r.tempRoot, "repopulse-git-")
	if err != nil {
		return nil, errors.New("create private Git directory")
	}
	s := &Session{reader: r, ctx: ctx, root: root, dir: filepath.Join(root, "objects.git"), data: domain.GitData{Repository: metadata, Refs: []domain.GitRef{}, Files: []domain.GitFile{}, Commits: []domain.GitCommit{}, Sources: []domain.SourceState{}}}
	success := false
	defer func() {
		if !success {
			_ = s.Close()
		}
	}()
	if err := os.Mkdir(filepath.Join(root, "hooks"), 0700); err != nil {
		return nil, err
	}
	acquire, cancel := context.WithTimeout(ctx, r.limits.Timeout)
	defer cancel()
	if _, err = s.command(acquire, "", 1<<20, "init", "--bare", "--template="+filepath.Join(root, "hooks"), s.dir); err != nil {
		return nil, err
	}
	transport := &transport{client: r.client, upstream: metadata.CanonicalURL + ".git", maximum: r.limits.Bytes}
	local, closeProxy, err := transport.serve(acquire)
	if err != nil {
		return nil, err
	}
	defer closeProxy()
	advertised, remoteErr := s.command(acquire, "", r.limits.Bytes, "ls-remote", "--symref", local, "HEAD", "refs/heads/*", "refs/tags/*")
	head, headSHA, refs, parseErr := parseRefs(advertised)
	s.data.Refs = refs
	if s.data.Repository.DefaultBranch == nil && strings.HasPrefix(head, "refs/heads/") {
		branch := strings.TrimPrefix(head, "refs/heads/")
		s.data.Repository.DefaultBranch = &branch
	}
	if s.data.Repository.CommitSHA == nil {
		sha := headSHA
		if s.data.Repository.DefaultBranch != nil {
			sha = ""
			for _, ref := range refs {
				if ref.Name == "refs/heads/"+*s.data.Repository.DefaultBranch {
					sha = ref.ObjectSHA
					break
				}
			}
		}
		if sha != "" {
			s.data.Repository.CommitSHA = &sha
		}
	}
	refCode := ""
	if remoteErr != nil {
		refCode = "GIT_FETCH_FAILED"
		if _, transportCode := transport.state(); transportCode != "" {
			refCode = transportCode
		}
	}
	if parseErr != nil {
		refCode = "GIT_REFS_INVALID"
	}
	if acquire.Err() != nil {
		refCode = "GIT_TIME_LIMIT"
	}
	refState := "unavailable"
	if len(refs) > 0 {
		refState = "partial"
	}
	s.data.Sources = append(s.data.Sources, source("refs", len(refs), refCode, refState))
	fetchCode := refCode
	if len(refs) > 0 || s.data.Repository.CommitSHA != nil {
		var input strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&input, "+%s:%s\n", ref.ObjectSHA, ref.Name)
		}
		if sha := s.data.Repository.CommitSHA; sha != nil {
			fmt.Fprintf(&input, "+%s:refs/repopulse/pinned\n", *sha)
		}
		// No depth/filter option: every announced head/tag is fetched in full.
		_, fetchErr := s.command(acquire, input.String(), 1<<20, "fetch", "--stdin", "--no-tags", "--no-recurse-submodules", "--no-auto-maintenance", "--no-write-fetch-head", local)
		if fetchErr != nil {
			fetchCode = "GIT_FETCH_FAILED"
		}
	}
	received, transportCode := transport.state()
	s.data.ReceivedBytes = received
	if transportCode != "" {
		fetchCode = transportCode
	}
	if acquire.Err() != nil {
		fetchCode = "GIT_TIME_LIMIT"
	}
	// The Git budget covers local object traversal as well as the fetch.
	s.readObjects(acquire, fetchCode)
	fingerprint := sha256.New()
	if s.data.Repository.CommitSHA != nil {
		_, _ = io.WriteString(fingerprint, *s.data.Repository.CommitSHA)
	}
	for _, ref := range s.data.Refs {
		_, _ = io.WriteString(fingerprint, ref.Name+"="+ref.ObjectSHA+"\n")
	}
	digest := hex.EncodeToString(fingerprint.Sum(nil))
	for i := range s.data.Sources {
		if s.data.Sources[i].Status == "complete" {
			s.data.Sources[i].Fingerprint = &digest
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return s, nil
}

func parseRefs(data []byte) (head, sha string, refs []domain.GitRef, err error) {
	refs = []domain.GitRef{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return head, sha, refs, errors.New("invalid refs")
		}
		if strings.HasPrefix(parts[0], "ref: ") && parts[1] == "HEAD" {
			head = strings.TrimPrefix(parts[0], "ref: ")
			continue
		}
		if !hash(parts[0]) {
			return head, sha, refs, errors.New("invalid object")
		}
		if parts[1] == "HEAD" {
			sha = parts[0]
			continue
		}
		if strings.HasSuffix(parts[1], "^{}") {
			continue
		}
		if !refName(parts[1]) || seen[parts[1]] {
			return head, sha, refs, errors.New("invalid ref")
		}
		seen[parts[1]] = true
		refs = append(refs, domain.GitRef{Name: parts[1], ObjectSHA: parts[0]})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return
}
func hash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func refName(s string) bool {
	if (!strings.HasPrefix(s, "refs/heads/") && !strings.HasPrefix(s, "refs/tags/")) || !utf8.ValidString(s) || strings.Contains(s, "..") || strings.Contains(s, "@{") || strings.ContainsAny(s, " \t\r\n\x00~^:?*[\\") {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}
func source(scope string, count int, code, failureState string) domain.SourceState {
	at := domain.UTC(time.Now())
	state := domain.SourceState{Source: "git", Scope: scope, Status: "complete", ObtainedAt: &at, Sample: domain.Sample{Collected: &count, Total: &count}}
	if code != "" {
		state.Status = failureState
		state.Sample.Total = nil
		state.Reason = &domain.Reason{Code: code, Message: "Сбор Git ограничен или прерван", CannotConclude: "Полнота зависимых показателей не подтверждена"}
	}
	return state
}

func (s *Session) readObjects(ctx context.Context, fetchCode string) {
	sha := s.data.Repository.CommitSHA
	if sha == nil {
		code := fetchCode
		if code == "" && len(s.data.Refs) > 0 {
			code = "GIT_REVISION_UNAVAILABLE"
		}
		if ctx.Err() != nil {
			code = "GIT_TIME_LIMIT"
		}
		s.data.Sources = append(s.data.Sources, source("tree", 0, code, "unavailable"))
	} else {
		kind, kindErr := s.command(ctx, "", 64, "cat-file", "-t", *sha)
		tree, err := s.command(ctx, "", 100<<20, "ls-tree", "-r", "-l", "-z", "--full-tree", *sha)
		treeCode := ""
		if err != nil {
			treeCode = "GIT_TREE_INTERRUPTED"
		}
		if kindErr != nil || strings.TrimSpace(string(kind)) != "commit" {
			treeCode = "GIT_REVISION_UNAVAILABLE"
			tree = nil
		}
		for _, record := range bytes.Split(tree, []byte{0}) {
			if ctx.Err() != nil {
				break
			}
			if len(record) == 0 {
				continue
			}
			header, path, ok := bytes.Cut(record, []byte{'\t'})
			fields := strings.Fields(string(header))
			if !ok || len(fields) != 4 || !hash(fields[2]) || !utf8.Valid(path) {
				treeCode = "GIT_TREE_INVALID"
				continue
			}
			if len(s.data.Files) >= s.reader.limits.Paths {
				treeCode = "GIT_PATH_LIMIT"
				break
			}
			size, _ := strconv.ParseInt(fields[3], 10, 64)
			s.data.Files = append(s.data.Files, domain.GitFile{Path: string(path), Mode: fields[0], ObjectSHA: fields[2], Size: size})
		}
		if ctx.Err() != nil {
			treeCode = "GIT_TIME_LIMIT"
		}
		state := "unavailable"
		if len(s.data.Files) > 0 {
			state = "partial"
		}
		s.data.Sources = append(s.data.Sources, source("tree", len(s.data.Files), treeCode, state))
	}

	history, err := s.command(ctx, "", 2<<20, "rev-list", "--timestamp", "--topo-order", "--max-count="+strconv.Itoa(s.reader.limits.Commits+1), "--all")
	historyCode := fetchCode
	if err != nil {
		historyCode = "GIT_HISTORY_INTERRUPTED"
	}
	for _, line := range strings.Split(strings.TrimSpace(string(history)), "\n") {
		if ctx.Err() != nil {
			break
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !hash(fields[1]) {
			historyCode = "GIT_HISTORY_INVALID"
			continue
		}
		seconds, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			historyCode = "GIT_HISTORY_INVALID"
			continue
		}
		if len(s.data.Commits) >= s.reader.limits.Commits {
			historyCode = "GIT_COMMIT_LIMIT"
			break
		}
		at := time.Unix(seconds, 0).UTC()
		if at.Year() < 1 || at.Year() > 9999 {
			historyCode = "GIT_HISTORY_INVALID"
			continue
		}
		s.data.Commits = append(s.data.Commits, domain.GitCommit{SHA: fields[1], CommittedAt: at})
	}
	if sha != nil && ctx.Err() == nil {
		main, mainErr := s.command(ctx, "", 2<<20, "rev-list", "--max-count="+strconv.Itoa(s.reader.limits.Commits+1), *sha)
		mainIDs := strings.Fields(string(main))
		mainComplete := mainErr == nil && len(mainIDs) <= s.reader.limits.Commits
		mainSet := map[string]bool{}
		for _, id := range mainIDs {
			if !hash(id) {
				mainComplete = false
				if historyCode == "" {
					historyCode = "GIT_HISTORY_INVALID"
				}
				continue
			}
			mainSet[id] = true
		}
		for i := range s.data.Commits {
			inMain := mainSet[s.data.Commits[i].SHA]
			// An observed SHA proves ancestry even in a partial walk. Its
			// absence proves non-membership only after a complete traversal.
			if inMain || mainComplete {
				s.data.Commits[i].Main = &inMain
			}
		}
		if len(mainIDs) > s.reader.limits.Commits && historyCode == "" {
			historyCode = "GIT_COMMIT_LIMIT"
		}
		if mainErr != nil && historyCode == "" {
			historyCode = "GIT_HISTORY_INTERRUPTED"
		}
	}
	if ctx.Err() != nil {
		historyCode = "GIT_TIME_LIMIT"
	}

	state := "unavailable"
	if len(s.data.Commits) > 0 {
		state = "partial"
	}
	s.data.Sources = append(s.data.Sources, source("history", len(s.data.Commits), historyCode, state))
}

func (s *Session) ReadBlob(ctx context.Context, objectSHA string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !hash(objectSHA) {
		return nil, &readError{"GIT_BLOB_UNAVAILABLE"}
	}
	if s.ctx.Err() != nil {
		return nil, &readError{"GIT_TIME_LIMIT"}
	}
	bounded, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	defer cancel()
	ctx = bounded
	kind, err := s.command(ctx, "", 64, "cat-file", "-t", objectSHA)
	if err != nil || strings.TrimSpace(string(kind)) != "blob" {
		return nil, &readError{"GIT_BLOB_UNAVAILABLE"}
	}
	sizeText, err := s.command(ctx, "", 64, "cat-file", "-s", objectSHA)
	size, parseErr := strconv.ParseInt(strings.TrimSpace(string(sizeText)), 10, 64)
	if err != nil || parseErr != nil || size < 0 {
		return nil, &readError{"GIT_BLOB_UNAVAILABLE"}
	}
	if size > 1<<20 || s.blobBytes+size > 200<<20 {
		return nil, &readError{"GIT_BLOB_LIMIT"}
	}
	contents, err := s.command(ctx, "", size, "cat-file", "blob", objectSHA)
	s.blobBytes += int64(len(contents))
	if err != nil {
		return nil, err
	}
	return contents, nil
}
