package domain

import "time"

// GitData contains transient structural facts, never file/patch contents or
// commit messages. A processor turns these into the normalized Snapshot contract.
type GitData struct {
	Repository  RepositoryMetadata
	Refs        []GitRef
	Files       []GitFile
	Commits     []GitCommit
	MainCommits []ActivityCommit
	// MainWalkComplete records a successful traversal of the pinned commit.
	// It can prove the last commit even when fetching other refs was incomplete.
	MainWalkComplete bool
	Sources          []SourceState
	ReceivedBytes    int64
}
type GitFile struct {
	Path, Mode, ObjectSHA string
	Size                  int64
}
type GitCommit struct {
	SHA         string
	CommittedAt time.Time
	// Main is nil when a bounded/interrupted traversal cannot establish
	// membership. False requires a complete walk of the main revision.
	Main *bool
}
