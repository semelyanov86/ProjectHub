// Package git inspects local Git working trees read-only: whether a directory
// is a repository, its current branch, and whether tracked files are modified.
package git

// State classifies the outcome of inspecting one directory.
type State string

// Inspection states.
const (
	// StateRepo means the directory is inside a Git working tree; Repo is populated.
	StateRepo State = "repo"
	// StateNotRepo means Git reported that the directory is not a repository.
	StateNotRepo State = "not_repo"
	// StateUnavailable means the git executable could not be found.
	StateUnavailable State = "unavailable"
	// StateError means inspection failed (timeout, permission, malformed output, ...).
	StateError State = "error"
)

// RepoInfo summarises Git repository state for a project directory.
type RepoInfo struct {
	// Branch is the current branch name; empty when Detached.
	Branch string `json:"branch"`
	// HeadSHA is the full commit hash of HEAD; empty for an unborn branch.
	HeadSHA string `json:"headSha"`
	// Detached reports a detached HEAD.
	Detached bool `json:"detached"`
	// Unborn reports a branch without commits (freshly initialised repository).
	Unborn bool `json:"unborn"`
	// IsDirty reports staged or unstaged changes to tracked files.
	IsDirty bool `json:"isDirty"`
	// DirtyUncertain reports that IsDirty may be a false positive: filter
	// drivers defined by the repository itself were disabled for safety, so
	// files they would normally convert can compare as modified.
	DirtyUncertain bool `json:"dirtyUncertain"`
	// Upstream is the configured upstream branch, if any.
	Upstream string `json:"upstream"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
}

// Inspection is the result of inspecting one directory.
type Inspection struct {
	State State    `json:"state"`
	Repo  RepoInfo `json:"repo"`
	// Message is a short human-readable reason for StateUnavailable/StateError.
	Message string `json:"message"`
}
