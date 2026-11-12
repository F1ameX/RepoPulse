package dependencies

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func hasReason(r result, code string) bool {
	for _, reason := range r.reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestNPMVersionsAndSections(t *testing.T) {
	manifest := []byte(`{"dependencies":{"lodash":"^4.0.0","@scope/pkg":"~2.0.0"},"devDependencies":{"lodash":"^4","dev":"*"},"optionalDependencies":{"opt":"*"},"peerDependencies":{"peer":"*"}}`)
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			lock := []byte(fmt.Sprintf(`{"lockfileVersion":%d,"packages":{"node_modules/lodash":{"version":"4.17.21"},"node_modules/@scope/pkg":{"version":"2.1.0-beta.1"},"node_modules/dev":{"version":"1.2.3"},"node_modules/opt":{"version":"3.0.0"},"node_modules/peer":{"version":"5.0.0"},"node_modules/dev/node_modules/transitive":{"version":"9.9.9"}}}`, version))
			got := parseNPM(manifest, lock)
			if len(got.reasons) != 0 || len(got.deps) != 5 {
				t.Fatalf("%+v", got)
			}
			for _, d := range got.deps {
				if d.ManifestPath != "package.json" || d.VersionSource != fmt.Sprintf("npm_lock_v%d", version) || !reflect.DeepEqual(d.Origins, []domain.DependencyOrigin{{ManifestPath: d.ManifestPath, VersionSource: d.VersionSource}}) {
					t.Fatalf("%+v", d)
				}
			}
			if got.deps[0].Package != "@scope/pkg" || got.deps[0].Version != "2.1.0-beta.1" || got.deps[2].Package != "lodash" || got.deps[2].Version != "4.17.21" {
				t.Fatalf("%+v", got.deps)
			}
		})
	}
}

func TestNPMIncompleteKeepsUsefulEntries(t *testing.T) {
	manifest := []byte(`{"dependencies":{"good":"^1","bad":"^4.17.0"}}`)
	for _, tc := range []struct {
		name, lock, code string
		count            int
	}{
		{"missing", "", "NPM_LOCK_MISSING", 0},
		{"broken", "{", "NPM_LOCK_INVALID", 0},
		{"v1", `{"lockfileVersion":1}`, "NPM_LOCK_UNSUPPORTED", 0},
		{"missing packages", `{"lockfileVersion":3}`, "NPM_LOCK_INVALID", 0},
		{"unresolved", `{"lockfileVersion":3,"packages":{"node_modules/good":{"version":"1.2.3"},"node_modules/bad":{"version":"^4.17.0"}}}`, "NPM_VERSION_UNRESOLVED", 1},
		{"missing selected", `{"lockfileVersion":2,"packages":{"node_modules/good":{"version":"1.2.3"}}}`, "NPM_VERSION_UNRESOLVED", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := parseNPM(manifest, []byte(tc.lock))
			if len(r.deps) != tc.count || !hasReason(r, tc.code) {
				t.Fatalf("%+v", r)
			}
			if tc.count == 1 && r.deps[0].Version != "1.2.3" {
				t.Fatal("manifest range used as version")
			}
		})
	}
	for _, selector := range []string{"npm:actual@1.0.0", "file:../pkg", "workspace:*", "github:user/repo"} {
		r := parseNPM([]byte(fmt.Sprintf(`{"dependencies":{"alias":%q}}`, selector)), []byte(`{"lockfileVersion":3,"packages":{"node_modules/alias":{"version":"1.0.0"}}}`))
		if len(r.deps) != 0 || !hasReason(r, "NPM_REFERENCE_UNSUPPORTED") {
			t.Fatalf("%+v", r)
		}
	}
}

func TestMavenLocalManagementPropertiesAndOrigins(t *testing.T) {
	r := parseMaven([]byte(`<project xmlns="http://maven.apache.org/POM/4.0.0">
 <properties><base>2.4.0</base><release>${base}</release><g>org.example</g></properties>
 <dependencyManagement><dependencies>
 <dependency><groupId>org.example</groupId><artifactId>lib</artifactId><version>1.0</version></dependency>
 <dependency><groupId>${g}</groupId><artifactId>lib</artifactId><type>test-jar</type><classifier>tests</classifier><version>${release}</version></dependency>
 <dependency><groupId>unused</groupId><artifactId>managed-only</artifactId><version>9.0</version></dependency>
 </dependencies></dependencyManagement>
 <dependencies>
 <dependency><groupId>${g}</groupId><artifactId>lib</artifactId><type>test-jar</type><classifier>tests</classifier><scope>test</scope><optional>true</optional></dependency>
 <dependency><groupId>org.example</groupId><artifactId>lib</artifactId><version>${release}</version><scope>provided</scope></dependency>
 </dependencies></project>`))
	if len(r.reasons) != 0 || len(r.deps) != 2 {
		t.Fatalf("%+v", r)
	}
	for _, d := range r.deps {
		if d.Package != "org.example:lib" || d.Version != "2.4.0" || d.ManifestPath != "pom.xml" {
			t.Fatalf("%+v", d)
		}
	}
	if r.deps[0].VersionSource != "maven_dependency_management" || r.deps[1].VersionSource != "maven_declaration" {
		t.Fatalf("%+v", r.deps)
	}
}

func TestMavenUnresolvedEntriesDoNotDropOthers(t *testing.T) {
	for _, version := range []string{"", "${external}", "[1.0,2.0)", "2.0-SNAPSHOT", "${cycle}"} {
		pom := fmt.Sprintf(`<project><properties><cycle>${cycle}</cycle></properties>
 <dependencyManagement><dependencies><dependency><groupId>vendor</groupId><artifactId>bom</artifactId><type>pom</type><scope>import</scope><version>3</version></dependency></dependencies></dependencyManagement>
 <parent><groupId>parent</groupId><artifactId>parent</artifactId><version>1</version></parent>
 <dependencies><dependency><groupId>x</groupId><artifactId>bad</artifactId><version>%s</version></dependency>
 <dependency><groupId>x</groupId><artifactId>good</artifactId><version>1.2.3</version><scope>runtime</scope><optional>true</optional></dependency></dependencies></project>`, version)
		r := parseMaven([]byte(pom))
		if len(r.deps) != 1 || r.deps[0].Package != "x:good" || !hasReason(r, "MAVEN_VERSION_UNRESOLVED") || !hasReason(r, "MAVEN_EXTERNAL_BOM") || !hasReason(r, "MAVEN_EXTERNAL_PARENT") {
			t.Fatalf("%+v", r)
		}
	}
	r := parseMaven([]byte("<project><dependencies>"))
	if !hasReason(r, "MAVEN_MANIFEST_INVALID") {
		t.Fatalf("%+v", r)
	}
	// Management entries with another type/classifier must not resolve this declaration.
	r = parseMaven([]byte(`<project><dependencyManagement><dependencies><dependency><groupId>x</groupId><artifactId>a</artifactId><type>test-jar</type><classifier>tests</classifier><version>1</version></dependency></dependencies></dependencyManagement><dependencies><dependency><groupId>x</groupId><artifactId>a</artifactId></dependency></dependencies></project>`))
	if len(r.deps) != 0 || !hasReason(r, "MAVEN_VERSION_UNRESOLVED") {
		t.Fatalf("%+v", r)
	}
}

func TestGoRequireDeclarations(t *testing.T) {
	r := parseGoMod([]byte("module example.com/root\n\ngo 1.26\nrequire example.com/single v1.2.3\nrequire (\n example.com/pre v1.0.0-rc.1\n example.com/pseudo v0.0.0-20260101000000-abcdef123456\n example.com/transitive v1.0.0 // indirect\n)\n"))
	if len(r.reasons) != 0 || len(r.deps) != 3 {
		t.Fatalf("%+v", r)
	}
	want := []string{"v1.2.3", "v1.0.0-rc.1", "v0.0.0-20260101000000-abcdef123456"}
	for i, d := range r.deps {
		if d.Version != want[i] || d.ManifestPath != "go.mod" || d.VersionSource != "go_require_declaration" || !strings.HasPrefix(d.Package, "example.com/") {
			t.Fatalf("%+v", d)
		}
	}
}

func TestGoInfluencingReplaceExclude(t *testing.T) {
	for _, tc := range []struct{ directive, code string }{
		{"replace example.com/lib => ../local", "GO_REPLACE"},
		{"replace example.com/lib v1.2.3 => example.com/fork v1.0.0", "GO_REPLACE"},
		{"exclude example.com/lib v1.2.3", "GO_EXCLUDE"},
		{"replace example.com/other => ../other", ""},
		{"replace example.com/lib v1.0.0 => ../other", ""},
		{"exclude example.com/lib v1.0.0", ""},
	} {
		r := parseGoMod([]byte("module example.com/root\nrequire example.com/lib v1.2.3\n" + tc.directive + "\n"))
		if len(r.deps) != 1 || r.deps[0].Version != "v1.2.3" {
			t.Fatalf("%+v", r)
		}
		if tc.code != "" && !hasReason(r, tc.code) || tc.code == "" && len(r.reasons) != 0 {
			t.Fatalf("%+v", r)
		}
	}
	if r := parseGoMod([]byte("require (")); !hasReason(r, "GO_MANIFEST_INVALID") {
		t.Fatalf("%+v", r)
	}
}

func TestGoInvalidRequireKeepsValidDeclarations(t *testing.T) {
	r := parseGoMod([]byte("module example.com/root\nrequire (\n example.com/bad latest\n example.com/good v1.2.3\n)\n"))
	if len(r.deps) != 1 || r.deps[0].Package != "example.com/good" || !hasReason(r, "GO_VERSION_UNRESOLVED") {
		t.Fatalf("%+v", r)
	}
}
