package core

import "context"

// TicketReader reads a project's tickets. The source package implements it
// by parsing docs/tickets/*.md.
type TicketReader interface {
	Tickets(ctx context.Context) ([]Ticket, error)
}

// GitState reads a project's git state: branches, merge status, and file
// contents on a branch. The git source implements it.
//
// A Source reads one project's raw state; git and github implement the
// parts they own, and derive composes them. This is designed so a gitlab
// source can join later (ADR-003).
type GitState interface {
	Branches(ctx context.Context) ([]string, error)
	IsMergedToMain(ctx context.Context, branch string) (bool, error)
	FileOnBranch(ctx context.Context, branch, path string) (string, error)
}

// PRReader reads a project's open pull requests, keyed by branch. The
// github source implements it.
type PRReader interface {
	OpenPRs(ctx context.Context) (map[string]PRState, error) // keyed by branch
}

// Dispatcher drives the routine /fire API and merge approval on explicit
// human action. No auto-anything on the server; autopilot lives in the
// routines, not here.
// Briefing is what a dispatch tells the routine beyond which ticket to work
// on: the agent's instructions, its skills, and any per-ticket refinement the
// operator added. It is passed by value and may be entirely zero — a project
// that has configured no agents dispatches with an empty Briefing, which is
// exactly the behaviour that existed before agents did.
type Briefing struct {
	// AgentName, Prompt and Skills come from the Agent configured for the
	// ticket's role, when one exists.
	AgentName string   `json:"agent_name,omitempty"`
	Prompt    string   `json:"prompt,omitempty"`
	Skills    []string `json:"skills,omitempty"`
	// Notes is the operator's per-ticket refinement — "use the v2 endpoint",
	// "don't touch the migration". Separate from Prompt because it is advice
	// about ONE ticket, while the prompt is how this agent always works.
	Notes string `json:"notes,omitempty"`
}

// Empty reports whether this briefing carries nothing worth sending.
func (b Briefing) Empty() bool {
	return b.AgentName == "" && b.Prompt == "" && b.Notes == "" && len(b.Skills) == 0
}

type Dispatcher interface {
	Fire(ctx context.Context, p Project, ticketID int, brief Briefing) (sessionURL string, err error)
	Autopilot(ctx context.Context, p Project) (on bool, err error)
	SetAutopilot(ctx context.Context, p Project, on bool) error
	ApproveMerge(ctx context.Context, p Project, prNumber int) error
}
