// runs.go persists what FlightDeck itself did: every routine it fired, for
// which ticket, and how that attempt ended.
//
// This is the one kind of history the derive engine cannot reconstruct. A
// ticket's status comes from git and is recomputed on every read (ADR-001);
// "this server ran routine R for ticket 7 at 12:04, and the branch never
// appeared" exists nowhere in git and is exactly what the scheduler needs to
// avoid firing ticket 7 again on its next tick. Recording it is not a cache
// of derived state — see ADR-007 for the full argument, and note that a run's
// `state` is a RUN's lifecycle, never a ticket's status.
package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RunState is where one dispatch attempt ended up.
type RunState string

const (
	// RunRunning: the routine was fired and its branch has not been observed
	// yet. This is the only state that blocks re-firing the same ticket.
	RunRunning RunState = "running"
	// RunObserved: a claude/NNN-* branch appeared for the ticket, so the
	// routine really picked the work up. Terminal, and the happy path — what
	// happens to the work afterwards is the derive engine's business, not a
	// run's.
	RunObserved RunState = "observed"
	// RunTimedOut: no branch appeared within the configured window. The
	// ticket becomes eligible for another attempt.
	RunTimedOut RunState = "timed_out"
	// RunFailed: the dispatch call itself errored, so nothing was started.
	RunFailed RunState = "failed"
)

// Run is one dispatch attempt.
type Run struct {
	ID         int64
	ProjectID  string
	TicketID   int
	Attempt    int // 1 for a ticket's first attempt, incrementing per retry
	State      RunState
	SessionURL string
	Detail     string // why it settled the way it did; empty while running
	StartedAt  time.Time
	SettledAt  time.Time // zero while State is RunRunning
}

// Active reports whether this run still blocks re-firing its ticket.
func (r Run) Active() bool { return r.State == RunRunning }

// StartRun records a dispatch that has just been fired and returns the stored
// Run, with its assigned ID and attempt number. attempt is the caller's — the
// scheduler derives it from LatestRun — so this method stays a plain write
// with no read-modify-write race of its own.
func (s *Store) StartRun(ctx context.Context, projectID string, ticketID, attempt int, sessionURL string, at time.Time) (Run, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (project_id, ticket_id, attempt, state, session_url, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, projectID, ticketID, attempt, string(RunRunning), sessionURL, formatTime(at))
	if err != nil {
		return Run{}, fmt.Errorf("recording run for project %q ticket %d: %w", projectID, ticketID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Run{}, fmt.Errorf("recording run for project %q ticket %d: %w", projectID, ticketID, err)
	}
	return Run{
		ID: id, ProjectID: projectID, TicketID: ticketID, Attempt: attempt,
		State: RunRunning, SessionURL: sessionURL, StartedAt: at,
	}, nil
}

// SettleRun moves a run out of RunRunning. Settling an already-settled run is
// a no-op rather than an error: two ticks racing to observe the same branch
// is normal, and the first answer is as good as the second.
func (s *Store) SettleRun(ctx context.Context, id int64, state RunState, detail string, at time.Time) error {
	if state == RunRunning {
		return fmt.Errorf("settling run %d: %q is not a settled state", id, state)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE runs SET state = ?, detail = ?, settled_at = ?
		WHERE id = ? AND state = ?
	`, string(state), detail, formatTime(at), id, string(RunRunning))
	if err != nil {
		return fmt.Errorf("settling run %d: %w", id, err)
	}
	return nil
}

// ActiveRuns returns every still-running dispatch for projectID, oldest
// first. This is what the scheduler reconciles against the board on each tick.
func (s *Store) ActiveRuns(ctx context.Context, projectID string) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, ticket_id, attempt, state, session_url, detail, started_at, settled_at
		FROM runs WHERE project_id = ? AND state = ? ORDER BY started_at, id
	`, projectID, string(RunRunning))
	if err != nil {
		return nil, fmt.Errorf("listing active runs for project %q: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()
	return scanRuns(rows)
}

// LatestRun returns the most recent run for one ticket, and whether there was
// one at all. The scheduler reads two things off it: Attempt (to number and
// cap retries) and State (to decide whether a retry is even allowed); the API
// reads SessionURL, so a link to a live session survives a restart.
func (s *Store) LatestRun(ctx context.Context, projectID string, ticketID int) (Run, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, ticket_id, attempt, state, session_url, detail, started_at, settled_at
		FROM runs WHERE project_id = ? AND ticket_id = ? ORDER BY id DESC LIMIT 1
	`, projectID, ticketID)

	run, err := scanRun(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Run{}, false, nil
	case err != nil:
		return Run{}, false, fmt.Errorf("reading latest run for project %q ticket %d: %w", projectID, ticketID, err)
	}
	return run, true, nil
}

// ProjectRuns returns a project's runs, most recent first, capped at limit.
// It backs the operator-facing history of what the scheduler has been doing.
func (s *Store) ProjectRuns(ctx context.Context, projectID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, ticket_id, attempt, state, session_url, detail, started_at, settled_at
		FROM runs WHERE project_id = ? ORDER BY id DESC LIMIT ?
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing runs for project %q: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()
	return scanRuns(rows)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface{ Scan(dest ...any) error }

func scanRun(sc rowScanner) (Run, error) {
	var (
		r                    Run
		state                string
		startedAt, settledAt string
	)
	if err := sc.Scan(&r.ID, &r.ProjectID, &r.TicketID, &r.Attempt, &state,
		&r.SessionURL, &r.Detail, &startedAt, &settledAt); err != nil {
		return Run{}, err
	}
	r.State = RunState(state)
	r.StartedAt = parseTime(startedAt)
	r.SettledAt = parseTime(settledAt)
	return r, nil
}

func scanRuns(rows *sql.Rows) ([]Run, error) {
	runs := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning run row: %w", err)
		}
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanning run rows: %w", err)
	}
	return runs, nil
}

// formatTime stores a timestamp as RFC3339 text, and the zero time as "" —
// SQLite has no time type, and an empty string reads back as "not settled".
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime is formatTime's inverse. An unparseable value reads back as the
// zero time rather than an error: a malformed timestamp must not be able to
// take a project's whole run history down with it.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// removeRuns deletes projectID's run history. Called from Remove, inside the
// same transaction that drops the project and its secrets.
func removeRuns(ctx context.Context, tx *sql.Tx, projectID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM runs WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("removing runs for project %q: %w", projectID, err)
	}
	return nil
}
