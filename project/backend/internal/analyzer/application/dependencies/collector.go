// Package dependencies extracts declared direct dependencies without running repository code.
package dependencies

import (
	"context"
	"sort"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const MaxDependencies = 20

type Collector struct{}

type result struct {
	deps    []domain.Dependency
	reasons []domain.Reason
}

func reason(code, message string) domain.Reason {
	return domain.Reason{Code: code, Message: message, CannotConclude: "Полнота прямых зависимостей не подтверждена; отсутствие уязвимостей не установлено"}
}
func (r *result) issue(code, message string) {
	for i, previous := range r.reasons {
		if previous.Code != code {
			continue
		}
		messages := strings.Split(previous.Message, "; ")
		for _, existing := range messages {
			if existing == message {
				return
			}
		}
		messages = append(messages, message)
		sort.Strings(messages)
		r.reasons[i].Message = strings.Join(messages, "; ")
		return
	}
	r.reasons = append(r.reasons, reason(code, message))
}

func dependency(ecosystem, name, version, manifest, source string) domain.Dependency {
	return domain.Dependency{Ecosystem: ecosystem, Package: name, Version: version, ManifestPath: manifest, VersionSource: source,
		Origins: []domain.DependencyOrigin{{ManifestPath: manifest, VersionSource: source}}}
}

func (Collector) Collect(ctx context.Context, _ domain.Analysis, session worker.GitSession) (snapshot.Collection, error) {
	if err := ctx.Err(); err != nil {
		return snapshot.Collection{}, err
	}
	if session == nil {
		return snapshot.Collection{}, domain.ErrInvalidData
	}
	git := session.Data()
	profiles := map[string]*result{"npm": {}, "maven": {}, "go": {}}
	npmFiles := make(map[string][]byte, 2)
	seen := map[string]bool{}
	present := map[string]bool{}
	failed := map[string]bool{}
	treeComplete := false
	var obtained *domain.SourceState
	for i := range git.Sources {
		if git.Sources[i].Source == "git" && git.Sources[i].Scope == "tree" {
			treeComplete = git.Sources[i].Status == "complete"
			obtained = &git.Sources[i]
		}
	}
	for _, file := range git.Files {
		if err := ctx.Err(); err != nil {
			return snapshot.Collection{}, err
		}
		profile := ""
		switch file.Path {
		case "package.json", "package-lock.json":
			profile = "npm"
		case "pom.xml":
			profile = "maven"
		case "go.mod", "go.work":
			profile = "go"
		default:
			continue
		}
		if seen[file.Path] {
			continue
		}
		seen[file.Path] = true
		present[profile] = true
		if file.Path == "go.work" {
			profiles[profile].issue("GO_WORKSPACE", "Корневой go.work ограничивает определение зависимостей по одному go.mod")
			continue
		}
		// Symlinks/submodules must not be interpreted as manifest contents.
		if file.Mode != "" && file.Mode != "100644" && file.Mode != "100755" {
			profiles[profile].issue("MANIFEST_UNSUPPORTED_MODE", "Манифест не является обычным файлом: "+file.Path)
			failed[file.Path] = true
			continue
		}
		contents, err := session.ReadBlob(ctx, file.ObjectSHA)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return snapshot.Collection{}, ctxErr
		}
		if err != nil {
			profiles[profile].issue("MANIFEST_READ_FAILED", "Не удалось прочитать "+file.Path)
			failed[file.Path] = true
			continue
		}
		switch file.Path {
		case "go.mod":
			parsed := parseGoMod(contents)
			profiles[profile].merge(parsed)
		case "pom.xml":
			parsed := parseMaven(contents)
			profiles[profile].merge(parsed)
		default:
			npmFiles[file.Path] = contents
		}
	}
	if seen["package.json"] && !failed["package.json"] {
		parsed := parseNPM(npmFiles["package.json"], npmFiles["package-lock.json"])
		// A read failure is distinct from an absent lock.
		if failed["package-lock.json"] {
			filtered := parsed.reasons[:0]
			for _, r := range parsed.reasons {
				if r.Code != "NPM_LOCK_MISSING" {
					filtered = append(filtered, r)
				}
			}
			parsed.reasons = filtered
		}
		profiles["npm"].merge(parsed)
	} else if seen["package-lock.json"] && !seen["package.json"] {
		profiles["npm"].issue("NPM_MANIFEST_MISSING", "Корневой package.json не найден")
	}
	if seen["go.work"] && !seen["go.mod"] {
		profiles["go"].issue("GO_MANIFEST_MISSING", "Корневой go.mod не найден")
	}
	collection := snapshot.Collection{}
	for _, profile := range []string{"npm", "maven", "go"} {
		r := profiles[profile]
		if !treeComplete {
			r.issue("DEPENDENCY_TREE_INCOMPLETE", "Полнота Git-дерева не подтверждена; наличие всех корневых файлов неизвестно")
		}
		state := domain.SourceState{Source: "git", Scope: "dependencies_" + profile, Status: "complete"}
		if obtained != nil {
			state.ObtainedAt = obtained.ObtainedAt
		}
		count := len(r.deps)
		state.Sample.Collected = &count
		if len(r.reasons) > 0 {
			state.Status = "partial"
			sort.Slice(r.reasons, func(i, j int) bool { return r.reasons[i].Code < r.reasons[j].Code })
			state.Reason = &r.reasons[0]
			// Separate scopes preserve every stable reason without storing parser diagnostics.
			for _, r := range r.reasons[1:] {
				collection.Sources = append(collection.Sources, domain.SourceState{Source: "git", Scope: state.Scope + "/" + r.Code, Status: "partial", Reason: &r, ObtainedAt: state.ObtainedAt})
			}
		} else if !present[profile] {
			state.Status = "not_applicable"
			r := reason("DEPENDENCY_MANIFEST_ABSENT", "Корневой манифест профиля не найден в полном Git-дереве")
			state.Reason = &r
		} else {
			state.Sample.Total = &count
		}
		collection.Sources = append(collection.Sources, state)
		collection.Dependencies = append(collection.Dependencies, r.deps...)
	}
	normalized, err := snapshot.NormalizeDependencies(collection.Dependencies)
	if err != nil {
		return snapshot.Collection{}, err
	}
	collection.Dependencies = normalized
	count := len(normalized)
	total := count
	aggregate := domain.SourceState{Source: "git", Scope: "dependencies", Status: "complete", Sample: domain.Sample{Collected: &count}}
	if obtained != nil {
		aggregate.ObtainedAt = obtained.ObtainedAt
	}
	for _, state := range collection.Sources {
		if state.Status == "partial" {
			aggregate.Status = "partial"
			r := reason("DEPENDENCY_PROFILE_INCOMPLETE", "Один или несколько профилей зависимостей собраны неполностью")
			aggregate.Reason = &r
			break
		}
	}
	if total > MaxDependencies {
		collection.Dependencies = normalized[:MaxDependencies]
		count = MaxDependencies
		aggregate.Status = "partial"
		r := reason("DEPENDENCY_LIMIT", "Превышен общий лимит 20 уникальных зависимостей; сохранена выборка в порядке ecosystem/package/version")
		aggregate.Reason = &r
	} else if aggregate.Status == "complete" {
		aggregate.Sample.Total = &total
	}
	if !present["npm"] && !present["maven"] && !present["go"] && treeComplete {
		aggregate.Status = "not_applicable"
		r := reason("DEPENDENCY_MANIFEST_ABSENT", "Поддерживаемые корневые манифесты не найдены")
		aggregate.Reason = &r
	}
	collection.Sources = append(collection.Sources, aggregate)
	if err := ctx.Err(); err != nil {
		return snapshot.Collection{}, err
	}
	return collection, nil
}

func (r *result) merge(other result) {
	r.deps = append(r.deps, other.deps...)
	for _, item := range other.reasons {
		r.issue(item.Code, item.Message)
	}
}
