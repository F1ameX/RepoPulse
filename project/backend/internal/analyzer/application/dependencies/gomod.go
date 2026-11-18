package dependencies

import (
	"errors"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

func parseGoMod(contents []byte) result {
	r := result{}
	file, err := modfile.Parse("go.mod", contents, nil)
	if err != nil {
		// Recover only semantic errors explicitly attributed to require lines by
		// modfile. A broken grammar cannot safely establish dependency boundaries.
		// Keeping the invalid version token lets modfile attribute version-major
		// errors to a require directive instead of a generic parseVersion error.
		_, classifiedErr := modfile.Parse("go.mod", contents, func(_ string, version string) (string, error) { return version, nil })
		var list modfile.ErrorList
		recoverable := errors.As(classifiedErr, &list) && len(list) > 0
		lines := strings.Split(string(contents), "\n")
		for _, item := range list {
			if item.Verb != "require" || item.Pos.Line < 1 || item.Pos.Line > len(lines) {
				recoverable = false
				break
			}
			lines[item.Pos.Line-1] = ""
		}
		if recoverable {
			file, err = modfile.Parse("go.mod", []byte(strings.Join(lines, "\n")), nil)
			if err == nil {
				r.issue("GO_VERSION_UNRESOLVED", "Некоторые require-записи исключены: нет допустимой конкретной версии")
			}
		}
	}
	if err != nil {
		r.issue("GO_MANIFEST_INVALID", "Корневой go.mod повреждён или имеет неподдерживаемый синтаксис")
		return r
	}
	for _, req := range file.Require {
		if req.Indirect {
			continue
		}
		if module.Check(req.Mod.Path, req.Mod.Version) != nil {
			r.issue("GO_VERSION_UNRESOLVED", "В require нет допустимой конкретной версии модуля")
			continue
		}
		r.deps = append(r.deps, dependency("Go", req.Mod.Path, req.Mod.Version, "go.mod", "go_require_declaration"))
		for _, replace := range file.Replace {
			if replace.Old.Path == req.Mod.Path && (replace.Old.Version == "" || replace.Old.Version == req.Mod.Version) {
				r.issue("GO_REPLACE", "replace влияет на прямую зависимость; сохранена версия декларации, а не разрешённый состав сборки")
			}
		}
		for _, exclude := range file.Exclude {
			if exclude.Mod == req.Mod {
				r.issue("GO_EXCLUDE", "exclude влияет на прямую зависимость; сохранена версия декларации, а не разрешённый состав сборки")
			}
		}
	}
	return r
}
