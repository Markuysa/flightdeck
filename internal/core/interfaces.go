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
type Dispatcher interface {
	Fire(ctx context.Context, p Project, ticketID int) (sessionURL string, err error)
	Autopilot(ctx context.Context, p Project) (on bool, err error)
	SetAutopilot(ctx context.Context, p Project, on bool) error
	ApproveMerge(ctx context.Context, p Project, prNumber int) error
}
