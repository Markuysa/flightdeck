// Package core defines FlightDeck's domain types and the interfaces every
// other package composes against. It imports no other internal package
// (ADR-004), so it is the single dependency-free root the rest of the
// codebase can build on without creating import cycles between features.
package core

// Project is a registered repository FlightDeck reads tickets and git/PR
// state from. Secrets (routine token, GitHub token) live in the registry,
// never here.
type Project struct {
	ID       string // stable slug
	Name     string
	RepoPath string // local checkout FlightDeck reads
	Remote   string // "github" | "" (local-only)
	Owner    string // github owner/repo, when Remote == "github"
	Repo     string
}
