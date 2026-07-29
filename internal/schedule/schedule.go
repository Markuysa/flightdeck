// Package schedule is the autonomous half of FlightDeck: a loop that reads
// each project's derived board and dispatches its ready tickets itself,
// respecting the dependency graph and a parallelism cap, until there is
// nothing left to start.
//
// # This reverses a founding constraint, deliberately
//
// Until now the server dispatched only on explicit human action — "no
// auto-anything on the server", stated in the dispatch and refresher package
// docs. That rule bought safety at the cost of the product's actual goal: a
// queue that drains without a human clicking each ticket. ADR-007 records the
// reversal and its limits. The important limit, preserved here: **the
// scheduler dispatches, it never merges.** Approving a merge stays a human
// action (or the routine's own autopilot behind a CI gate). Starting work an
// agent will open a PR for is recoverable; landing code on main is the step
// that is not, so it keeps its gate.
//
// # Why the run store is not optional
//
// Firing a routine does not change the board. The ticket stays `ready` until
// the agent pushes a claude/NNN-* branch, which can take minutes. A scheduler
// that looked only at the board would therefore re-fire the same ticket on
// every tick for as long as the agent took to start — a fork bomb aimed at
// your own agent budget. The runs table is what closes that window: a ticket
// with an active run is never a candidate, whatever the board says.
//
// # Dependencies
//
// This package imports internal/core and nothing else internal (ADR-004).
// Every collaborator below is a narrow interface that registry.Store and the
// api package's real types satisfy structurally, so the composition root can
// wire the concrete pieces without this package knowing they exist.
package schedule

import (
	"context"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

// BoardSource computes a project's derived board. api.ProjectSource satisfies
// this structurally.
type BoardSource interface {
	BoardTickets(ctx context.Context, p core.Project) ([]core.BoardTicket, error)
}

// ProjectLister lists registered projects. registry.Store satisfies it.
type ProjectLister interface {
	List(ctx context.Context) ([]core.Project, error)
}

// DispatcherFactory builds the dispatcher for one project.
// api.DispatcherFactory satisfies it.
type DispatcherFactory interface {
	Dispatcher(ctx context.Context, p core.Project) (core.Dispatcher, error)
}

// Run mirrors registry.Run's fields this package reads. It is redeclared
// rather than imported because internal/schedule may not import
// internal/registry (ADR-004); registry.Run's shape is what RunStore's
// implementation returns, and the composition root adapts between them.
type Run struct {
	ID         int64
	TicketID   int
	Attempt    int
	SessionURL string
	StartedAt  time.Time
}

// Outcome is how a run ended, as the scheduler decides it.
type Outcome string

const (
	// Observed: a branch appeared for the ticket, so the routine picked the
	// work up.
	Observed Outcome = "observed"
	// TimedOut: no branch appeared within the configured window.
	TimedOut Outcome = "timed_out"
	// Failed: the dispatch call itself errored.
	Failed Outcome = "failed"
)

// RunStore records what the scheduler dispatched. The composition root adapts
// registry.Store onto it.
type RunStore interface {
	// ActiveRuns returns projectID's runs that have not settled yet.
	ActiveRuns(ctx context.Context, projectID string) ([]Run, error)
	// LatestRun returns a ticket's most recent run, if any, plus whether it
	// is still active — enough to number a retry and to decide if one is due.
	LatestRun(ctx context.Context, projectID string, ticketID int) (run Run, active bool, found bool, err error)
	// StartRun records a dispatch that has just been fired.
	StartRun(ctx context.Context, projectID string, ticketID, attempt int, sessionURL string, at time.Time) (Run, error)
	// Settle moves a run out of the active state.
	Settle(ctx context.Context, runID int64, outcome Outcome, detail string, at time.Time) error
}

// BriefingReader assembles the agent briefing for a ticket — the prompt and
// skills configured for its role. The composition root adapts the registry
// onto it. A nil BriefingReader (or one that errors) means dispatches carry a
// bare briefing, which is exactly the pre-agents behaviour: the scheduler must
// never refuse to start work because the decoration could not be loaded.
type BriefingReader interface {
	Briefing(ctx context.Context, projectID, role string) core.Briefing
}

// AutopilotReader reports whether a project has opted into unattended work.
// core.Dispatcher already exposes this, so the scheduler reads it through the
// per-project dispatcher rather than inventing a second source of truth.
type AutopilotReader interface {
	Autopilot(ctx context.Context, p core.Project) (bool, error)
}

// Publisher notifies subscribers that something moved. api.Broker satisfies
// it; a nil Publisher is fine (Scheduler checks).
type Publisher interface {
	Publish(kind string, data map[string]any)
}

// Defaults for Config's zero values. They are deliberately conservative: the
// scheduler spends real agent budget without asking, so erring toward "too
// slow" is cheap and erring toward "too fast" is not.
const (
	DefaultMaxParallel = 2
	DefaultRunTimeout  = 30 * time.Minute
	DefaultMaxAttempts = 2
)

// EventDispatchStarted is published when the scheduler fires a ticket. It is
// the same event name handleDispatch publishes for a human-driven dispatch —
// a subscriber should not have to care which one started the work.
const EventDispatchStarted = "dispatch.started"

// Config tunes one Scheduler.
type Config struct {
	// Interval is how often to tick. <= 0 disables the scheduler entirely:
	// Run returns immediately.
	Interval time.Duration
	// MaxParallel caps how many tickets may be in flight per project at once
	// — counting both tickets the board already shows as in_progress and runs
	// this scheduler started that have not surfaced as a branch yet. <= 0
	// means DefaultMaxParallel.
	MaxParallel int
	// RunTimeout is how long a fired run may go without its branch appearing
	// before it is written off as timed out. <= 0 means DefaultRunTimeout.
	RunTimeout time.Duration
	// MaxAttempts caps dispatches per ticket. Once a ticket has burned this
	// many attempts without its branch appearing, the scheduler stops
	// retrying it and leaves it for a human. <= 0 means DefaultMaxAttempts.
	MaxAttempts int
}

func (c Config) maxParallel() int {
	if c.MaxParallel <= 0 {
		return DefaultMaxParallel
	}
	return c.MaxParallel
}

func (c Config) runTimeout() time.Duration {
	if c.RunTimeout <= 0 {
		return DefaultRunTimeout
	}
	return c.RunTimeout
}

func (c Config) maxAttempts() int {
	if c.MaxAttempts <= 0 {
		return DefaultMaxAttempts
	}
	return c.MaxAttempts
}
