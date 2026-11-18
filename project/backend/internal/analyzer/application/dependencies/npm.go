package dependencies

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

var npmName = regexp.MustCompile(`^(?:@[A-Za-z0-9._~-]+/)?[A-Za-z0-9._~-]+$`)
var npmVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func parseNPM(manifest, lock []byte) result {
	r := result{}
	var root map[string]json.RawMessage
	if json.Unmarshal(manifest, &root) != nil || root == nil {
		r.issue("NPM_MANIFEST_INVALID", "Корневой package.json повреждён или имеет неверную структуру")
		return r
	}
	names := map[string]string{}
	for _, section := range []string{"dependencies", "devDependencies", "optionalDependencies", "peerDependencies"} {
		raw, ok := root[section]
		if !ok {
			continue
		}
		var entries map[string]json.RawMessage
		if json.Unmarshal(raw, &entries) != nil || entries == nil {
			r.issue("NPM_MANIFEST_INVALID", "Раздел зависимостей package.json имеет неверную структуру")
			continue
		}
		for name, raw := range entries {
			var selector string
			if json.Unmarshal(raw, &selector) != nil || !npmName.MatchString(name) || selector == "" {
				r.issue("NPM_DEPENDENCY_INVALID", "Некоторые объявления npm-зависимостей имеют неверный формат")
				continue
			}
			// Unsupported references must not become OSV package names via aliases/links.
			if strings.Contains(selector, ":") || strings.Contains(selector, "/") {
				names[name] = "unsupported"
			} else if names[name] != "unsupported" {
				names[name] = selector
			}
		}
	}
	if len(lock) == 0 {
		r.issue("NPM_LOCK_MISSING", "Корневой package-lock.json отсутствует; точные версии npm неизвестны")
		return r
	}
	var locked struct {
		LockfileVersion int                        `json:"lockfileVersion"`
		Packages        map[string]json.RawMessage `json:"packages"`
	}
	if json.Unmarshal(lock, &locked) != nil {
		r.issue("NPM_LOCK_INVALID", "Корневой package-lock.json повреждён")
		return r
	}
	if locked.LockfileVersion != 2 && locked.LockfileVersion != 3 {
		r.issue("NPM_LOCK_UNSUPPORTED", "Поддерживаются только package-lock.json v2/v3")
		return r
	}
	if locked.Packages == nil {
		r.issue("NPM_LOCK_INVALID", "В package-lock.json отсутствует таблица packages")
		return r
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		if names[name] == "unsupported" {
			r.issue("NPM_REFERENCE_UNSUPPORTED", "npm-ссылки, aliases и workspace-зависимости не разрешаются этим профилем")
			continue
		}
		raw, found := locked.Packages["node_modules/"+name]
		var entry struct {
			Version string `json:"version"`
			Name    string `json:"name"`
			Link    bool   `json:"link"`
		}
		if !found || json.Unmarshal(raw, &entry) != nil || entry.Link || entry.Name != "" && entry.Name != name || !npmVersion.MatchString(entry.Version) || !semver.IsValid("v"+entry.Version) {
			r.issue("NPM_VERSION_UNRESOLVED", "Для части прямых npm-зависимостей нет точной версии в packages корневого lock-файла")
			continue
		}
		r.deps = append(r.deps, dependency("npm", name, entry.Version, "package.json", fmt.Sprintf("npm_lock_v%d", locked.LockfileVersion)))
	}
	return r
}
