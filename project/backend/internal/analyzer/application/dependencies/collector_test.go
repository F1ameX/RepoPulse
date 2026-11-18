package dependencies

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type testSession struct {
	data     domain.GitData
	blobs    map[string]string
	failures map[string]bool
	reads    map[string]int
	cancel   context.CancelFunc
}

func (s *testSession) Data() domain.GitData { return s.data }
func (s *testSession) Close() error         { return nil }
func (s *testSession) ReadBlob(ctx context.Context, sha string) ([]byte, error) {
	s.reads[sha]++
	if s.cancel != nil {
		s.cancel()
		return nil, ctx.Err()
	}
	if s.failures[sha] {
		return nil, errors.New("unsafe parser/upstream diagnostics")
	}
	return []byte(s.blobs[sha]), nil
}
func sessionWith(files map[string]string, complete bool) *testSession {
	s := &testSession{blobs: files, reads: map[string]int{}, failures: map[string]bool{}}
	status := "complete"
	var r *domain.Reason
	if !complete {
		status = "partial"
		v := reason("GIT_PATH_LIMIT", "Дерево неполное")
		r = &v
	}
	at := time.Now().UTC()
	s.data.Sources = []domain.SourceState{{Source: "git", Scope: "tree", Status: status, Reason: r, ObtainedAt: &at}}
	paths := []string{}
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		s.data.Files = append(s.data.Files, domain.GitFile{Path: path, ObjectSHA: path, Mode: "100644"})
	}
	return s
}
func collect(t *testing.T, s *testSession) snapshot.Collection {
	t.Helper()
	r, err := (Collector{}).Collect(t.Context(), domain.Analysis{}, s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func source(t *testing.T, c snapshot.Collection, scope string) domain.SourceState {
	t.Helper()
	for _, state := range c.Sources {
		if state.Scope == scope {
			return state
		}
	}
	t.Fatalf("missing source %s", scope)
	return domain.SourceState{}
}

func mixedFiles() map[string]string {
	return map[string]string{
		"package.json":        `{"dependencies":{"@scope/pkg":"^1"}}`,
		"package-lock.json":   `{"lockfileVersion":3,"packages":{"node_modules/@scope/pkg":{"version":"1.2.3"}}}`,
		"pom.xml":             `<project><dependencyManagement><dependencies><dependency><groupId>x</groupId><artifactId>lib</artifactId><version>2.0</version></dependency></dependencies></dependencyManagement><dependencies><dependency><groupId>x</groupId><artifactId>lib</artifactId><version>2.0</version></dependency><dependency><groupId>x</groupId><artifactId>lib</artifactId></dependency></dependencies></project>`,
		"go.mod":              "module example.com/root\nrequire example.com/lib v1.3.0\n",
		"nested/package.json": `{"dependencies":{"ignored":"1"}}`,
		"go.sum":              "not a selected version list",
	}
}
func TestCollectorMixedDedupOriginsAndSingleReads(t *testing.T) {
	s := sessionWith(mixedFiles(), true)
	c := collect(t, s)
	if len(c.Dependencies) != 3 || source(t, c, "dependencies").Status != "complete" {
		t.Fatalf("%+v", c)
	}
	maven := c.Dependencies[1]
	if maven.Ecosystem != "Maven" || maven.Package != "x:lib" || maven.Version != "2.0" || maven.ManifestPath != "pom.xml" || maven.VersionSource != "maven_declaration" ||
		!reflect.DeepEqual(maven.Origins, []domain.DependencyOrigin{{ManifestPath: "pom.xml", VersionSource: "maven_declaration"}, {ManifestPath: "pom.xml", VersionSource: "maven_dependency_management"}}) {
		t.Fatalf("%+v", maven)
	}
	for _, path := range []string{"go.mod", "pom.xml", "package.json", "package-lock.json"} {
		if s.reads[path] != 1 {
			t.Fatalf("reads: %+v", s.reads)
		}
	}
	if s.reads["go.sum"] != 0 || s.reads["nested/package.json"] != 0 {
		t.Fatalf("read outside root profiles: %+v", s.reads)
	}
	// File enumeration order must not affect selected dependencies or reasons.
	reversed := sessionWith(mixedFiles(), true)
	for i, j := 0, len(reversed.data.Files)-1; i < j; i, j = i+1, j-1 {
		reversed.data.Files[i], reversed.data.Files[j] = reversed.data.Files[j], reversed.data.Files[i]
	}
	if other := collect(t, reversed); !reflect.DeepEqual(c.Dependencies, other.Dependencies) {
		t.Fatal("order-dependent dependencies")
	}
}

func TestCollectorLimitAfterDeduplication(t *testing.T) {
	for _, n := range []int{20, 21} {
		files := mixedFiles()
		var pom strings.Builder
		pom.WriteString("<project><dependencies>")
		for i := n - 3; i >= 0; i-- {
			// Every declaration duplicated: limit counts unique tuples, not input records.
			for range 2 {
				fmt.Fprintf(&pom, "<dependency><groupId>x</groupId><artifactId>lib%02d</artifactId><version>1.0</version></dependency>", i)
			}
		}
		pom.WriteString("</dependencies></project>")
		files["pom.xml"] = pom.String()
		c := collect(t, sessionWith(files, true))
		state := source(t, c, "dependencies")
		if len(c.Dependencies) != 20 || *state.Sample.Collected != 20 {
			t.Fatalf("%+v", c)
		}
		if n == 20 && state.Status != "complete" || n == 21 && (state.Status != "partial" || state.Reason.Code != "DEPENDENCY_LIMIT") {
			t.Fatalf("%+v", state)
		}
		last := "@scope/pkg"
		if n == 21 {
			last = "x:lib18"
		}
		if c.Dependencies[0].Package != "example.com/lib" || c.Dependencies[19].Package != last {
			t.Fatal("unstable sample")
		}
	}
}

func TestCollectorReadFailureKeepsOtherProfiles(t *testing.T) {
	for _, path := range []string{"go.mod", "pom.xml", "package.json", "package-lock.json"} {
		t.Run(path, func(t *testing.T) {
			s := sessionWith(mixedFiles(), true)
			s.failures[path] = true
			c := collect(t, s)
			if len(c.Dependencies) != 2 || source(t, c, "dependencies").Status != "partial" {
				t.Fatalf("%+v", c)
			}
			found := false
			for _, state := range c.Sources {
				if state.Reason != nil {
					if strings.Contains(state.Reason.Message, "unsafe") {
						t.Fatal("raw error persisted")
					}
					if state.Reason.Code == "MANIFEST_READ_FAILED" {
						found = true
					}
				}
			}
			if !found {
				t.Fatal("read failure reason lost")
			}
			for _, n := range s.reads {
				if n != 1 {
					t.Fatal("retried a manifest")
				}
			}
		})
	}
}

func TestCollectorMissingTreeAndWorkspace(t *testing.T) {
	for _, complete := range []bool{false, true} {
		c := collect(t, sessionWith(nil, complete))
		state := source(t, c, "dependencies")
		want := "partial"
		if complete {
			want = "not_applicable"
		}
		if state.Status != want {
			t.Fatalf("%+v", state)
		}
	}
	files := mixedFiles()
	files["go.work"] = "go 1.26\nuse ./sub\n"
	s := sessionWith(files, true)
	c := collect(t, s)
	if source(t, c, "dependencies_go").Status != "partial" || source(t, c, "dependencies_go").Reason.Code != "GO_WORKSPACE" || len(c.Dependencies) != 3 || s.reads["go.work"] != 0 {
		t.Fatalf("%+v", c)
	}
	delete(files, "package-lock.json")
	c = collect(t, sessionWith(files, true))
	if source(t, c, "dependencies_npm").Reason.Code != "NPM_LOCK_MISSING" {
		t.Fatalf("%+v", c)
	}
}

func TestCollectorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := sessionWith(mixedFiles(), true)
	s.cancel = cancel
	_, err := (Collector{}).Collect(ctx, domain.Analysis{}, s)
	if !errors.Is(err, context.Canceled) || len(s.reads) != 1 {
		t.Fatalf("err=%v reads=%+v", err, s.reads)
	}
	_, err = (Collector{}).Collect(ctx, domain.Analysis{}, s)
	if !errors.Is(err, context.Canceled) || len(s.reads) != 1 {
		t.Fatal("read despite cancellation")
	}
}

func TestCollectorUnresolvedVersionKeepsOtherProfiles(t *testing.T) {
	files := mixedFiles()
	files["pom.xml"] = `<project><dependencies><dependency><groupId>x</groupId><artifactId>unknown</artifactId><version>${external}</version></dependency><dependency><groupId>x</groupId><artifactId>known</artifactId><version>3.2.1</version></dependency></dependencies></project>`
	c := collect(t, sessionWith(files, true))
	state := source(t, c, "dependencies_maven")
	if state.Status != "partial" || state.Reason.Code != "MAVEN_VERSION_UNRESOLVED" || source(t, c, "dependencies").Status != "partial" || len(c.Dependencies) != 3 {
		t.Fatalf("%+v", c)
	}
	for _, d := range c.Dependencies {
		if d.Package == "x:unknown" {
			t.Fatal("unresolved version leaked")
		}
		if d.Package == "x:known" && (d.Version != "3.2.1" || d.ManifestPath != "pom.xml" || d.VersionSource != "maven_declaration") {
			t.Fatalf("%+v", d)
		}
	}
}

func TestCollectorMalformedManifestKeepsOtherProfiles(t *testing.T) {
	for _, path := range []string{"pom.xml", "go.mod", "package.json", "package-lock.json"} {
		t.Run(path, func(t *testing.T) {
			files := mixedFiles()
			files[path] = "{not a manifest"
			c := collect(t, sessionWith(files, true))
			if len(c.Dependencies) != 2 || source(t, c, "dependencies").Status != "partial" {
				t.Fatalf("%+v", c)
			}
		})
	}
}

func TestCollectorEmptyProfilesAndUnsupportedMode(t *testing.T) {
	s := sessionWith(map[string]string{
		"package.json":      "{}",
		"package-lock.json": `{"lockfileVersion":3,"packages":{}}`,
		"pom.xml":           "<project/>",
		"go.mod":            "module example.com/root\n",
	}, true)
	c := collect(t, s)
	if len(c.Dependencies) != 0 || source(t, c, "dependencies").Status != "complete" || *source(t, c, "dependencies").Sample.Total != 0 {
		t.Fatalf("%+v", c)
	}
	s = sessionWith(map[string]string{"go.mod": "../outside"}, true)
	s.data.Files[0].Mode = "120000"
	c = collect(t, s)
	state := source(t, c, "dependencies_go")
	if state.Status != "partial" || state.Reason.Code != "MANIFEST_UNSUPPORTED_MODE" || len(s.reads) != 0 {
		t.Fatalf("%+v", c)
	}
}
