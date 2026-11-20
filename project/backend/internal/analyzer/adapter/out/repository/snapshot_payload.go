package repository

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Keep the existing snapshot JSON layout for old rows and store the normalized
// CI inputs in the same transaction, without adding them to the wire model.
type storedSnapshot struct {
	domain.Snapshot
	GitHubCI *domain.GitHubCI `json:"github_ci,omitempty"`
}

func decodeStoredSnapshot(body []byte) (domain.Snapshot, error) {
	var stored storedSnapshot
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil || decoder.Decode(new(any)) != io.EOF {
		return domain.Snapshot{}, domain.ErrInvalidData
	}
	stored.Snapshot.GitHubCI = stored.GitHubCI
	if err := stored.Snapshot.Validate(); err != nil {
		return domain.Snapshot{}, err
	}
	return stored.Snapshot, nil
}
