package repository_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/adapter/out/repository"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/storage"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestPublishedFindingGraphSurvivesCatalogChange(t *testing.T) {
	pool := testDatabase(t)
	ctx := t.Context()
	store := storage.NewService(repository.NewPostgres(pool))
	owner := domain.NewID()
	if _, err := pool.Exec(ctx, "INSERT INTO auth.principals(id,kind,created_at) VALUES ($1,'guest',now())", owner); err != nil {
		t.Fatal(err)
	}
	read := reportHTTPReader(t, pool, owner)
	s, r := loadFixtures(t)
	f := &r.Findings[0]
	e := domain.Evidence{ID: "support-doc-count", Source: "git", Kind: "aggregate", Description: "No supporting documents found"}
	f.EvidenceIDs = []string{e.ID}
	for i := range s.Inputs {
		if s.Inputs[i].ID == f.RuleID {
			s.Inputs[i].Evidence, s.Inputs[i].EvidenceTotal = []domain.Evidence{e}, new(int)
			*s.Inputs[i].EvidenceTotal = 1
			for _, category := range r.Categories {
				for j, m := range category.Metrics {
					if m.Input.ID == f.RuleID {
						category.Metrics[j].Input = s.Inputs[i]
					}
				}
			}
		}
	}
	var firstID string
	var firstBody []byte
	for _, ai := range []bool{false, true} {
		if ai {
			s.Versions.RecommendationCatalog += "-v2"
			r.Explanation.CatalogVersion = s.Versions.RecommendationCatalog
			r.Explanation.Mode, r.Explanation.Model, r.Explanation.FallbackReason = "ai_catalog_selection", text("control-model"), nil
			r.Recommendations[0].VariantID, r.Recommendations[0].Explanation = "expanded", "Expanded catalog explanation"
		}
		a, err := store.CreateAccepted(ctx, identity(owner, "guest"), domain.NewID(), s.Repository, s.Versions)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Start(ctx, a.ID, time.Minute); err != nil {
			t.Fatal(err)
		}
		result, err := store.Complete(ctx, a.ID, s.SnapshotData, r.ReportData)
		if err != nil {
			t.Fatal(err)
		}
		response := read(result.Report.ID, "owner")
		var got domain.Report
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &got) != nil {
			t.Fatalf("HTTP %d: %s", response.Code, response.Body)
		}
		if !reflect.DeepEqual(got.Findings, r.Findings) || !reflect.DeepEqual(got.Recommendations, r.Recommendations) ||
			!reflect.DeepEqual(got.Explanation, r.Explanation) || !got.CompletedAt.Equal(result.Report.CompletedAt) {
			t.Fatal("published finding, recommendation, explanation or date changed on read")
		}
		var linked []domain.Evidence
		for _, category := range got.Categories {
			for _, metric := range category.Metrics {
				if metric.Input.ID == f.RuleID {
					linked = metric.Input.Evidence
				}
			}
		}
		if !reflect.DeepEqual(linked, []domain.Evidence{e}) {
			t.Fatalf("unresolved evidence: %+v", linked)
		}
		if !ai {
			firstID, firstBody = got.ID, response.Body.Bytes()
		}
	}
	again := read(firstID, "owner")
	if again.Code != 200 || !bytes.Equal(again.Body.Bytes(), firstBody) {
		t.Fatal("refresh after publishing with a different catalog/provider changed the old report")
	}
}
