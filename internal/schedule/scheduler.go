package schedule

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

// Scheduler drains projects' ready queues on an interval. Build one with New
// and run it alongside the server; it stops when its context is canceled.
type Scheduler struct {
	cfg        Config
	projects   ProjectLister
	boards     BoardSource
	dispatcher DispatcherFactory
	runs       RunStore
	briefings  BriefingReader
	events     Publisher

	// now is the clock, swapped in tests to cross a timeout without sleeping.
	now func() time.Time
	// logf is the log sink, swapped in tests to keep output quiet.
	logf func(format string, args ...any)
}

// New returns a Scheduler. Every collaborator is required except events,
// which may be nil.
func New(cfg Config, projects ProjectLister, boards BoardSource, dispatcher DispatcherFactory, runs RunStore, briefings BriefingReader, events Publisher) *Scheduler {
	return &Scheduler{
		cfg:        cfg,
		projects:   projects,
		boards:     boards,
		dispatcher: dispatcher,
		runs:       runs,
		briefings:  briefings,
		events:     events,
		now:        time.Now,
		logf:       log.Printf,
	}
}

// Run ticks every cfg.Interval until ctx is canceled. It does nothing and
// returns immediately when the interval is <= 0 — the disabled configuration,
// which is the default: a server that dispatches on its own must be switched
// on deliberately (ADR-007).
func (s *Scheduler) Run(ctx context.Context) {
	if s.cfg.Interval <= 0 {
		return
	}
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Tick(ctx)
		}
	}
}

// Tick runs one full pass over every registered project. A project that fails
// mid-pass is logged and skipped; it never stops the others or the loop.
// Exported so tests (and a future manual "run now" control) can drive exactly
// one pass without a timer.
func (s *Scheduler) Tick(ctx context.Context) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		s.logf("flightdeck: schedule: listing projects: %v", err)
		return
	}
	for _, p := range projects {
		if err := s.tickProject(ctx, p); err != nil {
			s.logf("flightdeck: schedule: project %q: %v", p.ID, err)
		}
	}
}

// tickProject is one project's pass: reconcile what is already in flight,
// then start as much new work as the parallelism cap allows.
func (s *Scheduler) tickProject(ctx context.Context, p core.Project) error {
	d, err := s.dispatcher.Dispatcher(ctx, p)
	if err != nil {
		return fmt.Errorf("preparing dispatcher: %w", err)
	}

	// Autopilot is the switch. It already means "unattended progress is
	// allowed for this project" — it is what gates the routine's own
	// auto-merge — so reusing it keeps one switch in the UI rather than
	// growing a second one that means almost the same thing. A project whose
	// autopilot cannot be read (no .claude/autopilot.json) is treated as off:
	// unreadable must never mean "go ahead".
	on, err := d.Autopilot(ctx, p)
	if err != nil || !on {
		return nil //nolint:nilerr // an unreadable autopilot file means "off", not an error to report every tick
	}

	board, err := s.boards.BoardTickets(ctx, p)
	if err != nil {
		return fmt.Errorf("computing board: %w", err)
	}

	inFlight, err := s.reconcile(ctx, p, board)
	if err != nil {
		return err
	}

	slots := s.cfg.maxParallel() - inFlight
	if slots <= 0 {
		return nil
	}
	return s.startReady(ctx, p, d, board, slots)
}

// reconcile settles runs whose ticket has moved on, times out runs whose
// branch never appeared, and returns how many tickets are currently in
// flight for the project.
//
// In flight counts two disjoint things, and both matter:
//
//   - tickets the board shows as in_progress — an agent is demonstrably
//     working, whether this scheduler started it or a human did;
//   - runs still active after reconciliation — fired, but the branch has not
//     surfaced yet, so the board cannot see them.
//
// Counting only the first would let the cap be blown during the window
// between firing and the branch appearing, which is exactly the window the
// runs table exists to cover.
func (s *Scheduler) reconcile(ctx context.Context, p core.Project, board []core.BoardTicket) (int, error) {
	active, err := s.runs.ActiveRuns(ctx, p.ID)
	if err != nil {
		return 0, fmt.Errorf("listing active runs: %w", err)
	}

	status := make(map[int]core.DerivedStatus, len(board))
	for _, t := range board {
		status[t.ID] = t.Status
	}

	now := s.now()
	timeout := s.cfg.runTimeout()
	stillActive := 0
	for _, run := range active {
		// Anything but `ready` means the ticket left the queue: a branch
		// appeared (in_progress / in_review / needs_attention), or it landed
		// (done). Either way the routine did pick the work up, so the run has
		// served its purpose.
		if st, ok := status[run.TicketID]; ok && st != core.StatusReady {
			if err := s.runs.Settle(ctx, run.ID, Observed, fmt.Sprintf("ticket reached %s", st), now); err != nil {
				return 0, fmt.Errorf("settling run %d: %w", run.ID, err)
			}
			continue
		}
		if now.Sub(run.StartedAt) >= timeout {
			detail := fmt.Sprintf("no branch within %s", timeout)
			if err := s.runs.Settle(ctx, run.ID, TimedOut, detail, now); err != nil {
				return 0, fmt.Errorf("settling run %d: %w", run.ID, err)
			}
			s.logf("flightdeck: schedule: project %q ticket %d: %s (attempt %d)", p.ID, run.TicketID, detail, run.Attempt)
			continue
		}
		stillActive++
	}

	working := 0
	for _, t := range board {
		if t.Status == core.StatusInProgress {
			working++
		}
	}
	return working + stillActive, nil
}

// startReady fires up to slots ready tickets, lowest id first so a queue
// drains in the order it was written rather than in map order.
func (s *Scheduler) startReady(ctx context.Context, p core.Project, d core.Dispatcher, board []core.BoardTicket, slots int) error {
	ready := make([]core.BoardTicket, 0, len(board))
	for _, t := range board {
		if t.Status == core.StatusReady {
			ready = append(ready, t)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })

	for _, t := range ready {
		if slots <= 0 {
			return nil
		}
		attempt, eligible, err := s.nextAttempt(ctx, p.ID, t.ID)
		if err != nil {
			return err
		}
		if !eligible {
			continue
		}
		s.fire(ctx, p, d, t, attempt)
		slots--
	}
	return nil
}

// nextAttempt reports which attempt number a ticket's next dispatch would be,
// and whether it may be dispatched at all. A ticket is ineligible when a run
// is still active for it (already fired, branch not yet visible) or when it
// has burned every allowed attempt — at which point the scheduler stops and
// leaves it for a human rather than retrying forever.
func (s *Scheduler) nextAttempt(ctx context.Context, projectID string, ticketID int) (attempt int, eligible bool, err error) {
	last, active, found, err := s.runs.LatestRun(ctx, projectID, ticketID)
	if err != nil {
		return 0, false, fmt.Errorf("reading latest run for ticket %d: %w", ticketID, err)
	}
	if !found {
		return 1, true, nil
	}
	if active {
		return 0, false, nil
	}
	if last.Attempt >= s.cfg.maxAttempts() {
		return 0, false, nil
	}
	return last.Attempt + 1, true, nil
}

// fire dispatches one ticket and records the attempt. A dispatch failure is
// recorded as a settled, failed run rather than propagated: one project's
// broken routine must not stop the pass, and the failed attempt still counts
// against MaxAttempts so a permanently misconfigured project stops retrying.
func (s *Scheduler) fire(ctx context.Context, p core.Project, d core.Dispatcher, t core.BoardTicket, attempt int) {
	ticketID := t.ID

	// An autonomously started ticket gets the same briefing a human-dispatched
	// one would: same agent prompt, same skills. The two dispatch paths must
	// not produce differently-instructed agents for the same ticket.
	var brief core.Briefing
	if s.briefings != nil {
		brief = s.briefings.Briefing(ctx, p.ID, t.Role)
	}

	sessionURL, fireErr := d.Fire(ctx, p, ticketID, brief)
	now := s.now()

	run, err := s.runs.StartRun(ctx, p.ID, ticketID, attempt, sessionURL, now)
	if err != nil {
		// The routine may well be running now, and the record of it failed.
		// Log loudly: this is the one case where the scheduler could lose
		// track of work it started.
		s.logf("flightdeck: schedule: project %q ticket %d: dispatched but FAILED TO RECORD the run: %v",
			p.ID, ticketID, err)
		return
	}

	if fireErr != nil {
		if err := s.runs.Settle(ctx, run.ID, Failed, fireErr.Error(), now); err != nil {
			s.logf("flightdeck: schedule: project %q: settling failed run %d: %v", p.ID, run.ID, err)
		}
		s.logf("flightdeck: schedule: project %q ticket %d: dispatch failed (attempt %d/%d): %v",
			p.ID, ticketID, attempt, s.cfg.maxAttempts(), fireErr)
		return
	}

	s.logf("flightdeck: schedule: project %q ticket %d: dispatched (attempt %d)", p.ID, ticketID, attempt)
	if s.events != nil {
		s.events.Publish(EventDispatchStarted, map[string]any{
			"project_id":  p.ID,
			"ticket_id":   ticketID,
			"session_url": sessionURL,
			"scheduled":   true,
		})
	}
}
