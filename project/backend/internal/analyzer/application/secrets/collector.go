// Package secrets scans reachable history without retaining source contents.
package secrets

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const RulesVersion = "secrets-mvp-1.0.0"

type historyReader interface {
	HistoryTree(context.Context, string) ([]domain.GitFile, error)
}
type reasonError interface{ ReasonCode() string }

var assignment = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(?:password|passwd|secret|api_key|token)\s*[:=]\s*(?:"[^"\r\n]{8,128}"|'[^'\r\n]{8,128}')`)
var privateKey = regexp.MustCompile(`(?m)^-----BEGIN (?:PRIVATE KEY|RSA PRIVATE KEY|EC PRIVATE KEY|OPENSSH PRIVATE KEY)-----\r?$`)

// MatchTypes returns rule identifiers, never values or source lines.
func MatchTypes(data []byte) []string {
	kinds := []string{}
	if privateKey.Match(data) {
		kinds = append(kinds, "private_key_header")
	}
	if assignment.Match(data) {
		kinds = append(kinds, "suspicious_assignment")
	}
	return kinds
}

func Collect(ctx context.Context, a domain.Analysis, session worker.GitSession) snapshot.Collection {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	at := domain.UTC(time.Now())
	source := domain.SourceState{Source: "git", Scope: "secret_history", Status: "complete", ObtainedAt: &at}
	input := domain.MetricInput{ID: "SEC_SECRETS", Unit: "secret_scan", Status: "complete", ObservedAt: &at,
		Series: []int{}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{}}
	collected, matches := 0, 0
	exact := 0
	code := ""
	fail := func(reason string) {
		if code == "" {
			code = reason
		}
	}
	data := session.Data()
	for _, scope := range []string{"refs", "history"} {
		index := slices.IndexFunc(data.Sources, func(s domain.SourceState) bool { return s.Source == "git" && s.Scope == scope })
		if index < 0 || data.Sources[index].Status != "complete" {
			fail("SECRET_HISTORY_INCOMPLETE")
			if index >= 0 && data.Sources[index].Reason != nil {
				failCode := data.Sources[index].Reason.Code
				code = failCode
			}
		}
	}
	reader, ok := session.(historyReader)
	if !ok {
		fail("SECRET_HISTORY_READER_UNAVAILABLE")
	}
	if a.Versions.SecretRules != RulesVersion {
		fail("SECRET_RULES_UNSUPPORTED")
		ok = false
	}
	commits := append([]domain.GitCommit{}, data.Commits...)
	slices.SortFunc(commits, func(a, b domain.GitCommit) int {
		if a.SHA < b.SHA {
			return -1
		}
		if a.SHA > b.SHA {
			return 1
		}
		return 0
	})
	// Cache only match types per object. Repeated historical paths retain their revision.
	cache := map[string][]string{}
	bytesRead := int64(0)
	totalMatches := 0
scan:
	for _, commit := range commits {
		if !ok {
			break
		}
		if ctx.Err() != nil {
			fail("SECRET_TIME_LIMIT")
			break
		}
		files, err := reader.HistoryTree(ctx, commit.SHA)
		if err != nil {
			fail("SECRET_TREE_UNAVAILABLE")
			break
		}
		slices.SortFunc(files, func(a, b domain.GitFile) int {
			if a.Path < b.Path {
				return -1
			}
			if a.Path > b.Path {
				return 1
			}
			return 0
		})
		for _, file := range files {
			if ctx.Err() != nil {
				fail("SECRET_TIME_LIMIT")
				break scan
			}
			if file.Mode == "160000" {
				fail("SECRET_SUBMODULE_UNREAD")
				continue
			}
			if !domain.ValidEvidencePath(file.Path) {
				fail("SECRET_PATH_UNSUPPORTED")
				continue
			}
			kinds, seen := cache[file.ObjectSHA]
			if !seen {
				if file.Size < 0 {
					fail("SECRET_BLOB_UNREAD")
					continue
				}
				if file.Size > 1<<20 {
					fail("SECRET_BLOB_SIZE_LIMIT")
					break scan
				}
				if bytesRead+file.Size > 200<<20 {
					fail("SECRET_TOTAL_BYTES_LIMIT")
					break scan
				}
				body, err := session.ReadBlob(ctx, file.ObjectSHA)
				bytesRead += int64(len(body))
				if err != nil {
					reason := "SECRET_BLOB_UNREAD"
					if r, ok := err.(reasonError); ok {
						reason = r.ReasonCode()
					}
					fail(reason)
					break scan
				}
				if bytes.IndexByte(body, 0) >= 0 || !utf8.Valid(body) {
					fail("SECRET_BINARY_CONTENT")
					cache[file.ObjectSHA] = nil
					continue
				}
				kinds = MatchTypes(body)
				cache[file.ObjectSHA] = kinds
			}
			for _, kind := range kinds {
				matches++
				if kind == "private_key_header" {
					exact++
				}
				totalMatches++
				if len(input.Evidence) < 10 {
					name, sha, matchType := file.Path, commit.SHA, kind
					input.Evidence = append(input.Evidence, domain.Evidence{
						ID: fmt.Sprintf("SEC_SECRETS:%s:%s:%s", sha, name, kind), Source: "git", Kind: "secret_match",
						Description: "Potential secret: " + kind, Path: &name, Revision: &sha, SecretMatchType: &matchType,
					})
				}
			}
		}
		collected++
	}
	if ctx.Err() != nil {
		fail("SECRET_TIME_LIMIT")
	}
	source.Sample.Collected = &collected
	if code != "" {
		source.Status = "partial"
		source.Reason = &domain.Reason{Code: code, Message: "Скан истории на потенциальные секреты ограничен или прерван",
			CannotConclude: "Нельзя подтвердить отсутствие потенциальных секретов или рассчитать SEC_SECRETS по полному охвату"}
	} else {
		total := len(commits)
		source.Sample.Total = &total
		_ = input.RawValue.UnmarshalJSON([]byte(fmt.Sprint(matches)))
		input.EvidenceTotal = &totalMatches
		source.Sample.Numerator, source.Sample.Denominator = &exact, &matches
	}
	input.Status, input.Reason, input.Sample = source.Status, source.Reason, source.Sample
	input.EvidenceLimited = totalMatches > len(input.Evidence)
	return snapshot.Collection{Sources: []domain.SourceState{source}, Inputs: []snapshot.Input{{Value: input, Sources: []snapshot.Source{{Name: "git", Scope: "secret_history"}}}}}
}
