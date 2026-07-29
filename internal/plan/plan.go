// Package plan turns a plain-language goal into a graph of tickets: the step
// that was missing between "I want X" and a queue the scheduler can drain.
//
// # Why this is a proposal, not a write
//
// Planning never touches the repository. Propose returns a Plan for a human to
// read, edit, and approve; writing the tickets to disk is a separate, explicit
// call (apply.go). A model that could rewrite docs/tickets/ on its own would be
// able to redirect every agent the scheduler subsequently dispatches — the
// two-step shape is what keeps a bad plan a bad *suggestion*.
//
// # Validation is not optional
//
// The model returns a dependency graph, and a graph with a cycle or a dangling
// edge would poison the derive engine: `blocked` is computed from `depends`, so
// a cycle produces tickets that can never become ready and a dangling id
// produces one that is blocked forever on a ticket that does not exist. Validate
// (validate.go) rejects both before a Plan is ever returned. Structured outputs
// guarantee the JSON *shape*; nothing guarantees the graph is sane.
package plan

import (
	"context"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

// Roles are the ticket roles the planner may assign. They match the role field
// docs/tickets/*.md already carries, and they are what binds a ticket to an
// agent (internal/agents): a role the registry has no agent for still plans
// fine, it just dispatches with the default prompt.
var Roles = []string{"designer", "frontend", "backend", "qa", "dev"}

// Ticket is one planned ticket, before it exists on disk. It is deliberately
// not core.Ticket: that type describes a ticket the derive engine has read from
// a file, while this one has no id assigned yet — Number is the model's own
// 1-based index, used only to express dependencies within the proposal.
type Ticket struct {
	// Number is this ticket's position in the plan, 1-based. Depends refers to
	// other tickets by this number, not by any final ticket id, because the
	// final ids are only known at apply time (they continue the project's
	// existing numbering).
	Number int `json:"number"`
	// Title is the one-line summary that becomes the ticket file's `title:`.
	Title string `json:"title"`
	// Role names which kind of agent should implement it — one of Roles.
	Role string `json:"role"`
	// Depends lists the Numbers of tickets that must merge before this one can
	// start. The derive engine turns these into `blocked` until they are done
	// on main, so an inaccurate edge silently stalls the queue.
	Depends []int `json:"depends"`
	// Body is the ticket's markdown body: what to build and why.
	Body string `json:"body"`
	// Acceptance lists the criteria that make this ticket verifiably done. They
	// are rendered as a checklist in the ticket file — an agent with no
	// acceptance criteria has no definition of finished.
	Acceptance []string `json:"acceptance"`
	// Handoff is what the next ticket needs to know from this one (interfaces
	// introduced, decisions taken). Rendered as the `## Handoff` section the
	// git source already parses.
	Handoff string `json:"handoff"`
}

// Plan is a proposed decomposition, as returned to the operator for review.
type Plan struct {
	// Goal is the request this plan came from, echoed back.
	Goal string `json:"goal"`
	// Summary is the planner's one-paragraph account of its approach — the
	// thing to read before approving.
	Summary string `json:"summary"`
	// Tickets are in dependency order (a ticket never precedes one it depends
	// on), which is also the order they are written at apply time.
	Tickets []Ticket `json:"tickets"`
	// Model records which model produced the plan, and CreatedAt when.
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

// Planner turns a goal into a Plan. The API layer depends on this interface
// rather than the concrete client, so handler tests can plan without a network
// call or an API key.
type Planner interface {
	// Propose decomposes goal into tickets. existing is the project's current
	// tickets, passed so the plan can build on what is already queued rather
	// than duplicating it; it may be empty. Propose never writes anything.
	Propose(ctx context.Context, goal string, existing []core.Ticket) (Plan, error)
}
