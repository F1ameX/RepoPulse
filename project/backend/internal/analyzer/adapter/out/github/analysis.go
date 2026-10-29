package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// LookupForAnalysis confirms public identity and pins the default branch HEAD.
// It does not execute repository code or claim a complete collection snapshot.
func (c *Client) LookupForAnalysis(ctx context.Context, address domain.RepositoryURL) (domain.RepositoryMetadata, error) {
	confirmed, err := c.Lookup(ctx, address)
	if err != nil {
		return domain.RepositoryMetadata{}, err
	}
	r := confirmed.Repository
	result := domain.RepositoryMetadata{Platform: "github", GitHubID: r.GitHubID, CanonicalURL: r.CanonicalURL,
		Owner: r.Owner, Name: r.Name, DefaultBranch: r.DefaultBranch}
	if r.DefaultBranch == nil {
		return result, nil
	}
	body, status, _, err := c.get(ctx, "/repositories/"+r.GitHubID+"/commits/"+url.PathEscape(*r.DefaultBranch))
	if errors.Is(err, domain.ErrRepositoryUnavailable) {
		// A missing revision does not prove that the confirmed repository is private.
		return domain.RepositoryMetadata{}, domain.ErrRepositoryCheck
	}
	if err != nil {
		return domain.RepositoryMetadata{}, err
	}
	if status == http.StatusConflict {
		var empty struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &empty) == nil && empty.Message == "Git Repository is empty." {
			return result, nil
		}
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &commit) != nil || !commitPattern.MatchString(commit.SHA) {
		return domain.RepositoryMetadata{}, domain.ErrRepositoryCheck
	}
	result.CommitSHA = &commit.SHA
	return result, nil
}
