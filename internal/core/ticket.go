package core

// Ticket is one file-based ticket parsed from a project's docs/tickets/*.md.
//
// It deliberately has no Status field: status is derived from git and PR
// state on every read, never parsed as truth from the ticket file (ADR-001).
type Ticket struct {
	ID      int
	Title   string
	Role    string // designer|frontend|backend|qa|dev
	Depends []int
	Body    string
	Handoff string // the ## Handoff section, when present
}
