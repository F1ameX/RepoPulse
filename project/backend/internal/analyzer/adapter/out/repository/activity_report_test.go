package repository_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/activity"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	httpapi "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/in/http"
	reportclient "github.com/F1ameX/RepoPulse/project/backend/internal/api/adapter/out/report"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/grpcerr"
	reportgrpc "github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/in/grpc"
	reportrepo "github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/out/repository"
	reportapp "github.com/F1ameX/RepoPulse/project/backend/internal/report/application/report"
	reportpb "github.com/F1ameX/RepoPulse/project/backend/pkg/proto/report/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type reportResolver struct{ owner, foreign string }

func (v reportResolver) ResolveContext(_ context.Context, tokens apidomain.SessionTokens) (domain.VerifiedContext, error) {
	if tokens.GuestToken == nil {
		return domain.VerifiedContext{}, domain.ErrSessionRequired
	}
	id, expires := v.owner, time.Now().Add(time.Hour)
	switch *tokens.GuestToken {
	case "foreign":
		id = v.foreign
	case "expired":
		expires = time.Now().Add(-time.Second)
	}
	result := domain.VerifiedContext{GuestSessionID: &id, GuestExpiresAt: &expires}
	if *tokens.GuestToken == "signed-in" {
		result.UserID = &v.foreign
		result.AccountExpiresAt = &expires
	}
	return result, nil
}

func TestActivityReportGoldenVectorsPublishedAndReadWithoutRecollection(t *testing.T) {
	pool := testDatabase(t)
	pg := repository.NewPostgres(pool)
	store := storage.NewService(pg)
	fixture, _ := loadFixtures(t)
	owner := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	rpc := grpc.NewServer(grpc.UnaryInterceptor(grpcerr.UnaryServerInterceptor()))
	reportpb.RegisterReportServiceServer(rpc, reportgrpc.NewServer(reportapp.NewService(reportrepo.NewPostgres(pool))))
	listener := bufconn.Listen(1 << 20)
	go func() { _ = rpc.Serve(listener) }()
	defer rpc.Stop()
	conn, err := grpc.NewClient("passthrough:///report", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := reportclient.NewClient(conn, time.Second)
	handler := httpapi.NewHandler(nil, reportResolver{owner: owner, foreign: domain.NewID()}, httpapi.Options{Reports: reader})
	read := func(id, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/reports/"+id, nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "repopulse_guest", Value: cookie})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	paths, err := filepath.Glob(filepath.Join(backendRoot, "internal/analyzer/application/scoring/testdata/ACT-*.json"))
	if err != nil || len(paths) != 7 {
		t.Fatalf("golden fixtures: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var vector struct {
				Input struct {
					At      time.Time `json:"assessment_time"`
					Status  string    `json:"history_status"`
					Reason  string    `json:"history_reason"`
					Periods []struct {
						Commits int `json:"commits"`
					} `json:"periods"`
					Commits []struct {
						SHA  string    `json:"sha"`
						At   time.Time `json:"committer_date"`
						Main bool      `json:"is_main"`
					} `json:"commits"`
				} `json:"input"`
				Expected struct {
					Metrics map[string]struct {
						Raw    *float64 `json:"raw_value"`
						Score  *float64 `json:"normalized_score"`
						Status string   `json:"status"`
					} `json:"metrics"`
				} `json:"expected"`
			}
			if err := json.Unmarshal(body, &vector); err != nil {
				t.Fatal(err)
			}
			a := domain.Analysis{ID: domain.NewID(), SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: domain.Owner{Kind: "guest", ID: owner},
				Repository: fixture.Repository, Versions: fixture.Versions, AssessmentAt: vector.Input.At, CreatedAt: vector.Input.At,
				Window: domain.Window{From: vector.Input.At.Add(-140 * 24 * time.Hour), Until: vector.Input.At}, Status: "queued"}
			if len(vector.Input.Commits) == 0 && len(vector.Input.Periods) == 0 {
				a.Repository.DefaultBranch = nil
				a.Repository.CommitSHA = nil
			}
			source := domain.SourceState{Source: "git", Scope: domain.ActivityHistoryScope, Status: vector.Input.Status, ObtainedAt: &vector.Input.At}
			if source.Status != "complete" {
				source.Reason = &domain.Reason{Code: vector.Input.Reason, Message: "Обход истории прерван", CannotConclude: "Полная активность неизвестна"}
			}
			data := domain.GitData{Repository: a.Repository, Sources: []domain.SourceState{source}}
			withTree := strings.HasPrefix(filepath.Base(path), "ACT-01")
			withEmptyTree := strings.HasPrefix(filepath.Base(path), "ACT-03")
			withPartialTree := strings.HasPrefix(filepath.Base(path), "ACT-06")
			if withTree {
				data.Sources = append(data.Sources, domain.SourceState{Source: "git", Scope: "tree", Status: "complete", ObtainedAt: &vector.Input.At})
				for _, name := range []string{"README.md", "LICENSE", ".github/LICENSE", "src/main.go", "src/main_test.go"} {
					data.Files = append(data.Files, domain.GitFile{Path: name, Mode: "100644"})
				}
			}
			if withEmptyTree {
				zero := 0
				data.Sources = append(data.Sources, domain.SourceState{Source: "git", Scope: "tree", Status: "complete",
					Sample: domain.Sample{Collected: &zero, Total: &zero}})
			}
			if withPartialTree {
				one := 1
				data.Sources = append(data.Sources, domain.SourceState{Source: "git", Scope: "tree", Status: "partial",
					Sample: domain.Sample{Collected: &one}, Reason: &domain.Reason{
						Code: "GIT_PATH_LIMIT", Message: "Превышен лимит путей", CannotConclude: "Полное дерево неизвестно",
					}})
				data.Files = []domain.GitFile{{Path: "README.md", Mode: "100644"}}
			}
			// ACT-06's expected last-commit score assumes independent proof of
			// the pinned walk, as documented by the scoring golden-vector tests.
			data.MainWalkComplete = vector.Input.Status == "partial"
			for _, commit := range vector.Input.Commits {
				if !commit.Main {
					continue
				}
				// Golden fixtures use human-readable labels. Assign stable object
				// IDs for the evidence contract while retaining their actual dates.
				hash := sha1.Sum([]byte(commit.SHA))
				data.MainCommits = append(data.MainCommits, domain.ActivityCommit{SHA: hex.EncodeToString(hash[:]), CommittedAt: commit.At})
			}
			// ACT-04 specifies aggregate periods. Materialize an equivalent
			// trusted history spanning exactly those full weeks.
			if len(vector.Input.Periods) > 0 {
				start := a.AssessmentAt.Add(-time.Duration(len(vector.Input.Periods)) * 7 * 24 * time.Hour)
				for week, period := range vector.Input.Periods {
					for index := range period.Commits {
						data.MainCommits = append(data.MainCommits, domain.ActivityCommit{SHA: fmt.Sprintf("%040x", week*10000+index+1), CommittedAt: start.Add(time.Duration(week)*7*24*time.Hour + time.Duration(index)*time.Second)})
					}
				}
			}
			if _, err := pg.Create(t.Context(), a); err != nil {
				t.Fatal(err)
			}
			git := &snapshotSource{session: &snapshotSession{data: data}}
			runner := worker.NewService(pg, git, activity.NewProcessor(engine), store, time.Minute)
			if worked, err := runner.RunOnce(t.Context()); err != nil || !worked {
				t.Fatalf("worker: %v", err)
			}
			stored, err := store.GetResult(t.Context(), identity(owner, "guest"), a.ID)
			if err != nil {
				state, _ := pg.Get(t.Context(), a.ID)
				t.Fatalf("not published: %v %+v", err, state.Error)
			}
			response := read(stored.Report.ID, "owner")
			if response.Code != 200 {
				t.Fatalf("HTTP %d: %s", response.Code, response.Body)
			}
			var report domain.Report
			if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.ID != stored.Report.ID || report.SnapshotID != stored.Snapshot.ID || report.AnalysisID != a.ID || report.Versions != a.Versions || !reflect.DeepEqual(report.Repository, a.Repository) ||
				!report.AssessmentAt.Equal(a.AssessmentAt) || !report.CompletedAt.Equal(stored.Report.CompletedAt) || !report.Window.From.Equal(a.Window.From) {
				t.Fatal("stored metadata changed at HTTP boundary")
			}
			if report.Coverage.Percentage != math.Round(stored.Report.Coverage.Percentage*10)/10 || report.Coverage.Calculated != stored.Report.Coverage.Calculated {
				t.Fatal("public coverage is not rounded or its count changed")
			}
			if !reflect.DeepEqual(report.Sources, stored.Snapshot.Sources) {
				t.Fatal("source states, samples or metric links changed between storage and HTTP")
			}
			for i, category := range report.Categories {
				if category.Coverage.Percentage != math.Round(stored.Report.Categories[i].Coverage.Percentage*10)/10 {
					t.Fatal("public category coverage is not rounded")
				}
				if category.Explanation == "" {
					t.Fatalf("missing explanation for %s", category.ID)
				}
			}
			if strings.HasPrefix(filepath.Base(path), "ACT-06") && (report.Coverage.Percentage != 8.3 || report.Categories[0].Coverage.Percentage != 33.3 || stored.Report.Coverage.Percentage == 8.3) {
				t.Fatal("ACT-06 must expose 8.3%/33.3% while retaining storage precision")
			}
			for _, metric := range report.Categories[0].Metrics {
				want, exists := vector.Expected.Metrics[metric.Input.ID]
				if !exists {
					continue
				}
				raw, _ := metric.Input.RawValue.MarshalJSON()
				var value *float64
				if err := json.Unmarshal(raw, &value); err != nil {
					t.Fatal(err)
				}
				if !sameNumber(value, want.Raw) || metric.Input.Status != want.Status {
					t.Fatalf("%s raw=%s state=%s", metric.Input.ID, raw, metric.Input.Status)
				}
				var score *float64
				if want.Score != nil {
					rounded := math.Round(*want.Score*10) / 10
					score = &rounded
				}
				if !sameNumber(metric.NormalizedScore, score) {
					t.Fatalf("%s score=%v want %v", metric.Input.ID, metric.NormalizedScore, score)
				}
			}
			if withTree {
				docs, tests := report.Categories[1], report.Categories[2]
				share := docs.Metrics[1]
				test := tests.Metrics[0]
				if docs.ID != "documentation" || tests.ID != "testing" || report.Coverage.Calculated != 6 ||
					share.NormalizedScore == nil || *share.NormalizedScore != 33.3 || len(share.Input.Evidence) != 2 ||
					test.NormalizedScore == nil || *test.NormalizedScore != 100 || len(test.Input.Evidence) != 1 ||
					test.Input.Evidence[0].Path == nil || *test.Input.Evidence[0].Path != "src/main_test.go" {
					t.Fatalf("tree evidence or scores changed at HTTP boundary: docs=%+v tests=%+v", docs, tests)
				}
			}
			if withEmptyTree || withPartialTree {
				checkTreeDetails(t, report, withPartialTree)
			}
			if strings.Contains(response.Body.String(), owner) || !git.session.closed {
				t.Fatal("owner leaked or raw Git survived publication")
			}
			git.session.data.MainCommits = nil
			for _, cookie := range []string{"owner", "owner", "signed-in"} {
				again := read(report.ID, cookie)
				if again.Code != 200 || again.Body.String() != response.Body.String() || git.calls != 1 {
					t.Fatal("reading changed the result or queried Git")
				}
			}
			for cookie, status := range map[string]int{"foreign": 404, "expired": 401, "": 401} {
				if got := read(report.ID, cookie); got.Code != status {
					t.Fatalf("%s: %d want %d", cookie, got.Code, status)
				}
			}
			if got := read(strings.ToUpper(report.ID), "owner"); got.Code != 200 {
				t.Fatal("uppercase UUID rejected")
			}
			if got := read("bad-id", "owner"); got.Code != 404 {
				t.Fatal("invalid ID must be unavailable")
			}
		})
	}
	var snapshots int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM analyzer.snapshots").Scan(&snapshots); err != nil || snapshots != 7 {
		t.Fatalf("published %d snapshots: %v", snapshots, err)
	}
}

func checkTreeDetails(t *testing.T, report domain.Report, partial bool) {
	t.Helper()
	var tree *domain.SourceState
	for _, source := range report.Sources {
		if source.Source == "git" && source.Scope == "tree" {
			if tree != nil {
				t.Fatal("tree scope was duplicated")
			}
			tree = &source
		}
		if partial && source.Source == "git" && source.Scope == domain.ActivityHistoryScope &&
			!reflect.DeepEqual(source.MetricIDs, []string{"ACT_COMMIT_FREQ", "ACT_TREND"}) {
			t.Fatalf("independent last-commit proof was associated with the incomplete history: %v", source.MetricIDs)
		}
	}
	if tree == nil || !reflect.DeepEqual(tree.MetricIDs, []string{"DOC_README", "DOC_SUPPORT_SHARE", "TEST_PRESENCE"}) {
		t.Fatalf("missing links for the tree scope: %+v", tree)
	}
	if partial {
		if tree.Status != "partial" || tree.Reason == nil || tree.Reason.Code != "GIT_PATH_LIMIT" ||
			tree.Sample.Collected == nil || *tree.Sample.Collected != 1 || tree.Sample.Total != nil {
			t.Fatalf("incomplete tree lost its reason or known sample: %+v", tree)
		}
		for _, category := range report.Categories[1:3] {
			for _, metric := range category.Metrics {
				raw, _ := metric.Input.RawValue.MarshalJSON()
				if metric.Input.Status != "partial" || !reflect.DeepEqual(metric.Input.Reason, tree.Reason) ||
					metric.NormalizedScore != nil || string(raw) != "null" {
					t.Fatalf("incorrect details for partial metric %s", metric.Input.ID)
				}
			}
		}
		return
	}
	if tree.Status != "complete" || tree.Reason != nil || tree.Sample.Collected == nil || *tree.Sample.Collected != 0 ||
		tree.Sample.Total == nil || *tree.Sample.Total != 0 {
		t.Fatalf("complete empty tree became unknown: %+v", tree)
	}
	for _, metric := range report.Categories[1].Metrics {
		want := map[string]string{"DOC_README": "false", "DOC_SUPPORT_SHARE": "0"}[metric.Input.ID]
		raw, _ := metric.Input.RawValue.MarshalJSON()
		if string(raw) != want || metric.Input.Status != "complete" || metric.NormalizedScore == nil || *metric.NormalizedScore != 0 {
			t.Fatalf("confirmed %s value was lost: %s", metric.Input.ID, raw)
		}
	}
}

func TestActivityGitLimitsPersistFailureOrUsefulPartialReport(t *testing.T) {
	pool := testDatabase(t)
	pg := repository.NewPostgres(pool)
	store := storage.NewService(pg)
	fixture, _ := loadFixtures(t)
	owner := domain.NewID()
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ reason, code, message string }{
		{"GIT_TIME_LIMIT", "BUDGET_EXCEEDED", "Превышено время анализа"},
		{"GIT_SIZE_LIMIT", "ANALYSIS_FAILED", "Репозиторий превышает ограничения анализа"},
	} {
		for _, partial := range []bool{false, true} {
			name := tc.reason + "/empty"
			if partial {
				name = tc.reason + "/observed"
			}
			t.Run(name, func(t *testing.T) {
				a := domain.Analysis{ID: domain.NewID(), SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: domain.Owner{Kind: "guest", ID: owner},
					Repository: fixture.Repository, Versions: fixture.Versions, AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Status: "queued"}
				data := domain.GitData{Repository: a.Repository, Sources: []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "unavailable",
					Reason: &domain.Reason{Code: tc.reason, Message: "Прерван сбор Git", CannotConclude: "Полнота истории неизвестна"}}}}
				if partial {
					data.Sources[0].Status = "partial"
					data.MainCommits = []domain.ActivityCommit{{SHA: *a.Repository.CommitSHA, CommittedAt: a.AssessmentAt.Add(-time.Hour)}}
				}
				if _, err := pg.Create(t.Context(), a); err != nil {
					t.Fatal(err)
				}
				git := &snapshotSource{session: &snapshotSession{data: data}}
				runner := worker.NewService(pg, git, activity.NewProcessor(engine), store, time.Minute)
				if worked, err := runner.RunOnce(t.Context()); err != nil || !worked {
					t.Fatalf("worker: worked=%v err=%v", worked, err)
				}
				state, err := pg.Get(t.Context(), a.ID)
				if err != nil || !git.session.closed {
					t.Fatalf("state/cleanup: %v", err)
				}
				if !partial {
					if state.Status != "failed" || state.Error == nil || state.Error.Code != tc.code ||
						state.Error.Message != tc.message || state.ReportID != nil {
						t.Fatalf("limit reason was lost: %+v", state)
					}
					return
				}
				result, err := store.GetResult(t.Context(), identity(owner, "guest"), a.ID)
				if state.Status != "completed" || err != nil {
					t.Fatalf("useful partial result was lost: state=%+v err=%v", state, err)
				}
				if result.Report.Categories[0].Metrics[0].Input.Reason.Code != tc.reason || result.Report.Categories[0].Metrics[0].NormalizedScore != nil {
					t.Fatal("partial report lost the reason or invented a score")
				}
			})
		}
	}
}

func sameNumber(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 1e-9
}
