package dto

type ValidatedRepositoryDTO struct {
	Repository RepositoryDTO `json:"repository"`
	MovedFrom  *string       `json:"moved_from"`
}
