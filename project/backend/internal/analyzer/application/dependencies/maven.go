package dependencies

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type pomDependency struct {
	Group      string `xml:"groupId"`
	Artifact   string `xml:"artifactId"`
	Version    string `xml:"version"`
	Type       string `xml:"type"`
	Classifier string `xml:"classifier"`
	Scope      string `xml:"scope"`
	Optional   string `xml:"optional"`
}
type pomProperties map[string]string

func (p *pomProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = pomProperties{}
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &token); err != nil {
				return err
			}
			(*p)[token.Name.Local] = strings.TrimSpace(value)
		case xml.EndElement:
			if token.Name == start.Name {
				return nil
			}
		}
	}
}

type pomKey struct{ group, artifact, kind, classifier string }

func parseMaven(contents []byte) result {
	r := result{}
	var pom struct {
		XMLName      xml.Name        `xml:"project"`
		Properties   pomProperties   `xml:"properties"`
		Dependencies []pomDependency `xml:"dependencies>dependency"`
		Managed      []pomDependency `xml:"dependencyManagement>dependencies>dependency"`
		Parent       *struct{}       `xml:"parent"`
		Profiles     *struct{}       `xml:"profiles"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(contents))
	err := decoder.Decode(&pom)
	if err == nil {
		for {
			token, nextErr := decoder.Token()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				err = nextErr
				break
			}
			switch token := token.(type) {
			case xml.Comment:
			case xml.CharData:
				if len(bytes.TrimSpace(token)) != 0 {
					err = domain.ErrInvalidData
				}
			default:
				err = domain.ErrInvalidData
			}
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		r.issue("MAVEN_MANIFEST_INVALID", "Корневой pom.xml повреждён или имеет неверную структуру")
		return r
	}
	// Only local properties are available. Cycles and external references remain unresolved.
	var resolve func(string, map[string]bool) (string, bool)
	resolve = func(value string, visiting map[string]bool) (string, bool) {
		if len(visiting) > 64 || len(value) > 1<<20 {
			return "", false
		}
		value = strings.TrimSpace(value)
		for strings.Contains(value, "${") {
			start := strings.Index(value, "${")
			end := strings.Index(value[start:], "}")
			if end < 0 {
				return "", false
			}
			end += start
			key := value[start+2 : end]
			property, ok := pom.Properties[key]
			if !ok || visiting[key] {
				return "", false
			}
			visiting[key] = true
			expanded, ok := resolve(property, visiting)
			delete(visiting, key)
			if !ok {
				return "", false
			}
			if len(value)+len(expanded) > 1<<20 {
				return "", false
			}
			value = value[:start] + expanded + value[end+1:]
		}
		return value, true
	}
	expand := func(value string) (string, bool) { return resolve(value, map[string]bool{}) }
	keyOf := func(d pomDependency) (pomKey, bool) {
		group, g := expand(d.Group)
		artifact, a := expand(d.Artifact)
		kind, k := expand(d.Type)
		classifier, c := expand(d.Classifier)
		if kind == "" {
			kind = "jar"
		}
		return pomKey{group, artifact, kind, classifier}, g && a && k && c && group != "" && artifact != "" && !strings.ContainsAny(group+artifact+kind+classifier, " \t\r\n:")
	}
	managed := map[pomKey]string{}
	conflicts := map[pomKey]bool{}
	for _, d := range pom.Managed {
		scope, ok := expand(d.Scope)
		if ok && scope == "import" {
			r.issue("MAVEN_EXTERNAL_BOM", "Импортируемый BOM не разрешается локальным профилем")
			continue
		}
		key, ok := keyOf(d)
		if !ok {
			continue
		}
		version, ok := expand(d.Version)
		if !ok {
			version = ""
		}
		if previous, exists := managed[key]; exists && previous != version {
			conflicts[key] = true
		}
		managed[key] = version
	}
	if pom.Parent != nil {
		r.issue("MAVEN_EXTERNAL_PARENT", "Внешний parent не разрешается; сохранены только доступные локальные декларации")
	}
	if pom.Profiles != nil {
		r.issue("MAVEN_PROFILES_UNSUPPORTED", "Активация Maven profiles не моделируется")
	}
	for _, d := range pom.Dependencies {
		key, ok := keyOf(d)
		scope, scopeOK := expand(d.Scope)
		optional, optionalOK := expand(d.Optional)
		if !ok || !scopeOK || !optionalOK || optional != "" && optional != "true" && optional != "false" {
			r.issue("MAVEN_DEPENDENCY_UNRESOLVED", "Некоторые координаты или параметры прямых Maven-зависимостей не разрешены")
			continue
		}
		switch scope {
		case "", "compile", "provided", "runtime", "test", "system":
		default:
			r.issue("MAVEN_SCOPE_UNSUPPORTED", "Неподдерживаемый scope прямой Maven-зависимости")
			continue
		}
		source := "maven_declaration"
		version, ok := expand(d.Version)
		if d.Version == "" {
			version = managed[key]
			ok = !conflicts[key] && version != ""
			source = "maven_dependency_management"
		}
		if !ok || version == "" || strings.ContainsAny(version, "[](),$ \t\r\n") || strings.Contains(strings.ToUpper(version), "SNAPSHOT") || version == "LATEST" || version == "RELEASE" {
			r.issue("MAVEN_VERSION_UNRESOLVED", "Версия Maven-зависимости требует внешних данных, содержит диапазон, SNAPSHOT или неразрешённое свойство")
			continue
		}
		r.deps = append(r.deps, dependency("Maven", key.group+":"+key.artifact, version, "pom.xml", source))
	}
	return r
}
