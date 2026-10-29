package analysis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type memoryStore struct {
	mu          sync.Mutex
	submissions map[string]domain.Submission
	analyses    map[string]domain.Analysis
	readErr     error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{submissions: map[string]domain.Submission{}, analyses: map[string]domain.Analysis{}}
}

func (m *memoryStore) ReserveSubmission(_ context.Context, s domain.Submission) (domain.Submission, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := s.Owner.ID + "/" + s.RequestID
	if previous, ok := m.submissions[key]; ok {
		return previous, false, nil
	}
	m.submissions[key] = s
	return s, true, nil
}

func (m *memoryStore) AcceptSubmission(ctx context.Context, a domain.Analysis) (domain.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.Analysis{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := a.Owner.ID + "/" + a.RequestID
	s := m.submissions[key]
	s.AnalysisID, s.State = &a.ID, "accepted"
	m.submissions[key] = s
	m.analyses[a.ID] = a
	return a, nil
}

func (m *memoryStore) RejectSubmission(_ context.Context, id string, failure domain.SubmissionFailure) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, s := range m.submissions {
		if s.ID == id {
			s.Failure, s.State = &failure, "rejected"
			m.submissions[key] = s
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *memoryStore) Get(_ context.Context, id string) (domain.Analysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return domain.Analysis{}, m.readErr
	}
	if a, ok := m.analyses[id]; ok {
		return a, nil
	}
	return domain.Analysis{}, domain.ErrNotFound
}

type sourceFunc func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error)

func (f sourceFunc) LookupForAnalysis(ctx context.Context, address domain.RepositoryURL) (domain.RepositoryMetadata, error) {
	return f(ctx, address)
}

func fixture() (domain.VerifiedContext, domain.RepositoryMetadata, domain.Versions) {
	id, expiry := domain.NewID(), time.Now().Add(time.Hour)
	return domain.VerifiedContext{GuestSessionID: &id, GuestExpiresAt: &expiry},
		domain.RepositoryMetadata{Platform: "github", GitHubID: "42", CanonicalURL: "https://github.com/owner/repo", Owner: "Owner", Name: "Repo"},
		domain.Versions{Schema: "schema", Methodology: "method", ScoringConfig: "score", CollectorProfile: "profile",
			SecretRules: "secrets", RecommendationCatalog: "catalog", ExplanationSchema: "explanation", Prompt: "prompt"}
}

func TestReplayPrecedesMutableGitHubAndPreservesTerminalResult(t *testing.T) {
	verified, repository, versions := fixture()
	store := newMemoryStore()
	var calls int
	service := NewService(store, sourceFunc(func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error) {
		calls++
		if calls > 1 {
			return domain.RepositoryMetadata{}, domain.ErrRepositoryUnavailable
		}
		return repository, nil
	}), versions, time.Second)
	command := CreateAnalysisCommand{Context: verified, Request: CreateAnalysisRequest{RequestID: domain.NewID(), RepositoryURL: " https://GitHub.com/Owner/Repo.git/ "}}
	first, err := service.CreateAnalysis(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	first.Status, first.Error = "failed", &domain.AnalysisFailure{Code: "BUDGET_EXCEEDED"}
	store.analyses[first.ID] = first
	command.Request.RepositoryURL = repository.CanonicalURL
	again, err := service.CreateAnalysis(t.Context(), command)
	if err != nil || again.ID != first.ID || again.Status != "failed" || again.Error == nil || again.Error.Code != "BUDGET_EXCEEDED" || calls != 1 {
		t.Fatalf("replay=%+v error=%#v calls=%d", again, err, calls)
	}
	command.Request.RepositoryURL = "https://github.com/owner/other"
	if _, err := service.CreateAnalysis(t.Context(), command); !errors.Is(err, domain.ErrSubmissionChanged) || calls != 1 {
		t.Fatalf("changed URL error=%#v calls=%d", err, calls)
	}
	command.Request.RepositoryURL, command.Request.RequestID = repository.CanonicalURL, domain.NewID()
	if _, err := service.CreateAnalysis(t.Context(), command); !errors.Is(err, domain.ErrRepositoryUnavailable) || calls != 2 {
		t.Fatalf("new submission error=%#v calls=%d", err, calls)
	}
	if store.analyses[first.ID].Status != "failed" || len(store.analyses) != 1 {
		t.Fatal("new attempt changed the previous terminal analysis")
	}
	if _, err := service.CreateAnalysis(t.Context(), command); !errors.Is(err, domain.ErrRepositoryUnavailable) || calls != 2 {
		t.Fatalf("rejected submission was rechecked: %#v calls=%d", err, calls)
	}
}

func TestConcurrentDeliveryAndDisconnectedBrowserCreateOneJob(t *testing.T) {
	verified, repository, versions := fixture()
	store := newMemoryStore()
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	service := NewService(store, sourceFunc(func(ctx context.Context, _ domain.RepositoryURL) (domain.RepositoryMetadata, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return repository, ctx.Err()
		case <-ctx.Done():
			return domain.RepositoryMetadata{}, ctx.Err()
		}
	}), versions, 5*time.Second)
	command := CreateAnalysisCommand{Context: verified, Request: CreateAnalysisRequest{RequestID: domain.NewID(), RepositoryURL: repository.CanonicalURL}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan domain.Analysis, 1)
	failure := make(chan error, 1)
	go func() {
		a, err := service.CreateAnalysis(ctx, command)
		result <- a
		failure <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("source was not reached")
	}
	if _, err := service.CreateAnalysis(t.Context(), command); !errors.Is(err, domain.ErrSubmissionPending) {
		t.Fatalf("concurrent delivery: %#v", err)
	}
	cancel() // The first HTTP response is lost while its submission is in flight.
	close(release)
	first := <-result
	if err := <-failure; err != nil {
		t.Fatal(err)
	}
	again, err := service.CreateAnalysis(t.Context(), command)
	if err != nil || again.ID != first.ID || len(store.analyses) != 1 || calls.Load() != 1 {
		t.Fatalf("lost response replay=%+v error=%#v calls=%d", again, err, calls.Load())
	}
}

func TestNewExplicitSubmissionNeverClaimsUnconfirmedReuse(t *testing.T) {
	verified, repository, versions := fixture()
	store := newMemoryStore()
	service := NewService(store, sourceFunc(func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error) { return repository, nil }), versions, time.Second)
	command := CreateAnalysisCommand{Context: verified, Request: CreateAnalysisRequest{RequestID: domain.NewID(), RepositoryURL: repository.CanonicalURL}}
	first, err := service.CreateAnalysis(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	command.Request.RequestID = domain.NewID()
	second, err := service.CreateAnalysis(t.Context(), command)
	if err != nil || first.ID == second.ID || len(store.analyses) != 2 {
		t.Fatalf("unconfirmed reuse: %+v %#v", second, err)
	}
}

func TestStatusReadsEnforceOwnerAndNeverChangeAnalysis(t *testing.T) {
	verified, repository, versions := fixture()
	store := newMemoryStore()
	service := NewService(store, sourceFunc(func(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error) { return repository, nil }), versions, time.Second)
	a, err := service.CreateAnalysis(t.Context(), CreateAnalysisCommand{Context: verified,
		Request: CreateAnalysisRequest{RequestID: domain.NewID(), RepositoryURL: repository.CanonicalURL}})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		got, err := service.GetAnalysis(t.Context(), GetAnalysisQuery{Context: verified, AnalysisID: a.ID})
		if err != nil || got.ID != a.ID || len(store.analyses) != 1 {
			t.Fatalf("status read: %+v %#v", got, err)
		}
	}
	foreign, _, _ := fixture()
	for _, query := range []GetAnalysisQuery{{Context: foreign, AnalysisID: a.ID}, {Context: verified, AnalysisID: domain.NewID()}, {Context: verified, AnalysisID: "invalid"}} {
		if _, err := service.GetAnalysis(t.Context(), query); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("resource existence disclosed: %#v", err)
		}
	}
	userID, expires := domain.NewID(), time.Now().Add(time.Hour)
	both := verified
	both.UserID, both.AccountExpiresAt = &userID, &expires
	if _, err := service.GetAnalysis(t.Context(), GetAnalysisQuery{Context: both, AnalysisID: a.ID}); err != nil {
		t.Fatalf("original guest lost access after login: %#v", err)
	}
	store.readErr = errors.New("database unavailable")
	if _, err := service.GetAnalysis(t.Context(), GetAnalysisQuery{Context: verified, AnalysisID: a.ID}); err == nil || store.analyses[a.ID].Status != "queued" {
		t.Fatal("failed status read changed computation state")
	}
}
