package domain

import "time"

// GitData contains transient structural facts, never file/patch contents or
// commit messages. A processor turns these into the normalized Snapshot contract.
type GitData struct {
	Repository    RepositoryMetadata
	Refs          []GitRef
	Files         []GitFile
	Commits       []GitCommit
	Sources       []SourceState
	ReceivedBytes int64
}
type GitFile struct {
	Path, Mode, ObjectSHA string
	Size                  int64
}
type GitCommit struct {
	SHA         string
	CommittedAt time.Time
	Main        bool
}
