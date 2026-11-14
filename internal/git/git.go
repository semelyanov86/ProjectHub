// Package git defines data structures for inspecting local Git repository
// metadata. Repository inspection commands will be added in a later stage.
package git

// RepoInfo summarizes basic Git repository state for a project directory.
type RepoInfo struct {
	IsRepo    bool   `json:"isRepo"`
	Branch    string `json:"branch"`
	HeadSHA   string `json:"headSha"`
	IsDirty   bool   `json:"isDirty"`
	Ahead     int    `json:"ahead"`
	Behind    int    `json:"behind"`
	RemoteURL string `json:"remoteUrl"`
}
