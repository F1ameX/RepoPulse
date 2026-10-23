package dto

type RepositoryDTO struct {
	Platform      string  `json:"platform"`
	GitHubID      string  `json:"github_id"`
	CanonicalURL  string  `json:"canonical_url"`
	Owner         string  `json:"owner"`
	Name          string  `json:"name"`
	DefaultBranch *string `json:"default_branch"`
	CommitSHA     *string `json:"commit_sha"`
}
