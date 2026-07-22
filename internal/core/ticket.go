package core

// Ticket is one file-based ticket parsed from a project's docs/tickets/*.md.
//
// It deliberately has no Status field: status is derived from git and PR
// state on every read, never parsed as truth from the ticket file (ADR-001).
type Ticket struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Role    string `json:"role"` // designer|frontend|backend|qa|dev
	Depends []int  `json:"depends"`
	Body    string `json:"body"`
	Handoff string `json:"handoff"` // the ## Handoff section, when present
}
