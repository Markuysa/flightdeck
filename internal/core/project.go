// Package core defines FlightDeck's domain types and the interfaces every
// other package composes against. It imports no other internal package
// (ADR-004), so it is the single dependency-free root the rest of the
// codebase can build on without creating import cycles between features.
package core

// Project is a registered repository FlightDeck reads tickets and git/PR
// state from. Secrets (routine token, GitHub token) live in the registry,
// never here.
type Project struct {
	ID       string `json:"id"` // stable slug
	Name     string `json:"name"`
	RepoPath string `json:"repo_path"` // local checkout FlightDeck reads
	Remote   string `json:"remote"`    // "github" | "" (local-only)
	Owner    string `json:"owner"`     // github owner/repo, when Remote == "github"
	Repo     string `json:"repo"`
}
