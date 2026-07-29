package registry

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

var runAt = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

func TestStartRunIsActiveUntilSettled(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	run, err := store.StartRun(ctx, "acme", 7, 1, "https://claude.ai/code/s/1", runAt)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.ID == 0 {
		t.Error("StartRun returned a run with no ID")
	}
	if !run.Active() {
		t.Errorf("a freshly started run has state %q, want it active", run.State)
	}

	active, err := store.ActiveRuns(ctx, "acme")
	if err != nil {
		t.Fatalf("ActiveRuns: %v", err)
	}
	if len(active) != 1 || active[0].TicketID != 7 {
		t.Fatalf("ActiveRuns = %+v, want the one run for ticket 7", active)
	}
	if active[0].SessionURL != "https://claude.ai/code/s/1" {
		t.Errorf("ActiveRuns session URL = %q, want it preserved", active[0].SessionURL)
	}
	if !active[0].StartedAt.Equal(runAt) {
		t.Errorf("StartedAt round-tripped as %s, want %s", active[0].StartedAt, runAt)
	}

	settledAt := runAt.Add(2 * time.Minute)
	if err := store.SettleRun(ctx, run.ID, RunObserved, "branch appeared", settledAt); err != nil {
		t.Fatalf("SettleRun: %v", err)
	}

	active, err = store.ActiveRuns(ctx, "acme")
	if err != nil {
		t.Fatalf("ActiveRuns after settle: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("ActiveRuns after settling = %+v, want none", active)
	}

	latest, ok, err := store.LatestRun(ctx, "acme", 7)
	if err != nil || !ok {
		t.Fatalf("LatestRun: %v (found=%v)", err, ok)
	}
	if latest.State != RunObserved || latest.Detail != "branch appeared" {
		t.Errorf("LatestRun = %+v, want observed with its detail", latest)
	}
	if !latest.SettledAt.Equal(settledAt) {
		t.Errorf("SettledAt = %s, want %s", latest.SettledAt, settledAt)
	}
}

// TestSettleRunIsIdempotent: two scheduler ticks can race to observe the same
// branch. The second must not error, and must not overwrite the first answer.
func TestSettleRunIsIdempotent(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	run, err := store.StartRun(ctx, "acme", 7, 1, "", runAt)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.SettleRun(ctx, run.ID, RunObserved, "first", runAt.Add(time.Minute)); err != nil {
		t.Fatalf("first SettleRun: %v", err)
	}
	if err := store.SettleRun(ctx, run.ID, RunTimedOut, "second", runAt.Add(time.Hour)); err != nil {
		t.Fatalf("second SettleRun must be a no-op, not an error: %v", err)
	}

	latest, _, err := store.LatestRun(ctx, "acme", 7)
	if err != nil {
		t.Fatalf("LatestRun: %v", err)
	}
	if latest.State != RunObserved || latest.Detail != "first" {
		t.Errorf("re-settling overwrote the first answer: %+v", latest)
	}
}

func TestSettleRunRejectsRunningAsATarget(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()
	run, err := store.StartRun(ctx, "acme", 1, 1, "", runAt)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.SettleRun(ctx, run.ID, RunRunning, "", runAt); err == nil {
		t.Error("SettleRun(RunRunning) = nil, want an error — that is not a settled state")
	}
}

// TestLatestRunTracksAttempts: retry numbering is read off the latest run, so
// it has to be the newest one, not merely any one.
func TestLatestRunTracksAttempts(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	first, _ := store.StartRun(ctx, "acme", 7, 1, "", runAt)
	if err := store.SettleRun(ctx, first.ID, RunTimedOut, "no branch", runAt.Add(time.Hour)); err != nil {
		t.Fatalf("SettleRun: %v", err)
	}
	if _, err := store.StartRun(ctx, "acme", 7, 2, "", runAt.Add(2*time.Hour)); err != nil {
		t.Fatalf("second StartRun: %v", err)
	}

	latest, ok, err := store.LatestRun(ctx, "acme", 7)
	if err != nil || !ok {
		t.Fatalf("LatestRun: %v (found=%v)", err, ok)
	}
	if latest.Attempt != 2 || latest.State != RunRunning {
		t.Errorf("LatestRun = attempt %d/%s, want attempt 2 still running", latest.Attempt, latest.State)
	}
}

func TestLatestRunForUnknownTicket(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	_, ok, err := store.LatestRun(context.Background(), "acme", 999)
	if err != nil {
		t.Fatalf("LatestRun for an unknown ticket errored: %v", err)
	}
	if ok {
		t.Error("LatestRun reported a run for a ticket that was never dispatched")
	}
}

// TestActiveRunsAreScopedToOneProject: the scheduler reconciles per project,
// so a run leaking across projects would let one project's dispatch block
// another's ticket of the same number.
func TestActiveRunsAreScopedToOneProject(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if _, err := store.StartRun(ctx, "acme", 7, 1, "", runAt); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := store.StartRun(ctx, "other", 7, 1, "", runAt); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	active, err := store.ActiveRuns(ctx, "acme")
	if err != nil {
		t.Fatalf("ActiveRuns: %v", err)
	}
	if len(active) != 1 || active[0].ProjectID != "acme" {
		t.Errorf("ActiveRuns(acme) = %+v, want only acme's run", active)
	}
}

// TestRunsSurviveReopen is the point of persisting runs at all: the in-memory
// store this replaces forgot every session URL on restart, and a scheduler
// that forgot its in-flight runs would re-fire every ticket it had just
// dispatched.
func TestRunsSurviveReopen(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "registry.db")
	ctx := context.Background()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.StartRun(ctx, "acme", 7, 1, "https://claude.ai/code/s/9", runAt); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	active, err := reopened.ActiveRuns(ctx, "acme")
	if err != nil {
		t.Fatalf("ActiveRuns after reopen: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("ActiveRuns after reopen = %+v, want the in-flight run to survive", active)
	}
	if active[0].SessionURL != "https://claude.ai/code/s/9" {
		t.Errorf("session URL after reopen = %q, want it preserved", active[0].SessionURL)
	}
}

// TestRemoveProjectDropsItsRuns: leaving runs behind would let a re-registered
// project inherit a stale in-flight run and refuse to dispatch its ticket.
func TestRemoveProjectDropsItsRuns(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.Add(ctx, testProject("acme")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := store.StartRun(ctx, "acme", 7, 1, "", runAt); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.Remove(ctx, "acme"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	active, err := store.ActiveRuns(ctx, "acme")
	if err != nil {
		t.Fatalf("ActiveRuns: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("runs survived their project's removal: %+v", active)
	}
}

func TestProjectRunsIsMostRecentFirst(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		if _, err := store.StartRun(ctx, "acme", i, 1, "", runAt.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("StartRun: %v", err)
		}
	}

	runs, err := store.ProjectRuns(ctx, "acme", 2)
	if err != nil {
		t.Fatalf("ProjectRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("ProjectRuns(limit 2) returned %d runs, want 2", len(runs))
	}
	if runs[0].TicketID != 3 || runs[1].TicketID != 2 {
		t.Errorf("ProjectRuns order = %d then %d, want 3 then 2 (most recent first)", runs[0].TicketID, runs[1].TicketID)
	}
}
