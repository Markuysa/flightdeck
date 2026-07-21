package core

// DerivedStatus is a ticket's computed status. It is never stored — the
// derive package recomputes it from tickets, git, and PR state on every
// read (ADR-001).
type DerivedStatus string

// The set of statuses the derive engine can produce, mirroring the queue
// model in the template's docs/tickets/README.md.
const (
	StatusReady          DerivedStatus = "ready"
	StatusInProgress     DerivedStatus = "in_progress"
	StatusInReview       DerivedStatus = "in_review"
	StatusBlocked        DerivedStatus = "blocked"
	StatusNeedsAttention DerivedStatus = "needs_attention"
	StatusDone           DerivedStatus = "done"
)

// BoardTicket is a Ticket annotated with its derived status and, when
// applicable, the branch and PR carrying the work.
type BoardTicket struct {
	Ticket
	Status DerivedStatus
	Branch string   // claude/NNN-*, when it exists
	PR     *PRState // when in review
}

// PRState is the open pull request for a ticket's branch, when one exists.
type PRState struct {
	Number int
	URL    string
	CI     string // pending|green|red|unknown
}
