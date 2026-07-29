package schedule

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

var t0 = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

type harness struct {
	sched  *Scheduler
	runs   *fakeRunStore
	disp   *fakeDispatcher
	boards *fakeBoards
	events *fakePublisher
	briefs *fakeBriefings
	clock  time.Time
}

// newHarness wires a Scheduler over fakes, with autopilot ON and one
// registered project ("acme") unless a test changes it.
func newHarness(t *testing.T, cfg Config) *harness {
	t.Helper()
	h := &harness{
		runs:   newFakeRunStore(),
		disp:   &fakeDispatcher{autopilotOn: true},
		boards: &fakeBoards{},
		events: &fakePublisher{},
		briefs: &fakeBriefings{},
		clock:  t0,
	}
	h.sched = New(cfg,
		fakeProjects{list: []core.Project{{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}}},
		h.boards, fakeFactory{d: h.disp}, h.runs, h.briefs, h.events)
	h.sched.now = func() time.Time { return h.clock }
	h.sched.logf = func(string, ...any) {} // keep test output quiet
	return h
}

func (h *harness) tick() { h.sched.Tick(context.Background()) }

// TestFiresReadyTicketsUpToParallelismCap is the core behaviour: the queue
// drains by itself, but never more than MaxParallel at a time.
func TestFiresReadyTicketsUpToParallelismCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2})
	h.boards.set("acme", []core.BoardTicket{
		ticket(1, core.StatusReady),
		ticket(2, core.StatusReady),
		ticket(3, core.StatusReady),
	})

	h.tick()

	if got, want := h.disp.firedTickets(), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — lowest ids first, capped at MaxParallel", got, want)
	}
}

// TestDoesNotRefireATicketWhoseBranchHasNotAppearedYet is the bug the runs
// table exists to prevent. Firing does not change the board — the ticket stays
// `ready` until the agent pushes a branch — so a board-only scheduler would
// re-fire it on every single tick.
func TestDoesNotRefireATicketWhoseBranchHasNotAppearedYet(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 4})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()
	// The board is unchanged: the agent has not pushed anything yet.
	h.clock = h.clock.Add(5 * time.Second)
	h.tick()
	h.clock = h.clock.Add(5 * time.Second)
	h.tick()

	if got, want := h.disp.firedTickets(), []int{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v across three ticks, want exactly %v", got, want)
	}
}

// TestActiveRunsCountTowardTheCap: during the window between firing and the
// branch appearing the board still shows `ready`, so the cap has to count
// active runs too — otherwise it is blown wide open exactly when it matters.
func TestActiveRunsCountTowardTheCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2})
	h.boards.set("acme", []core.BoardTicket{
		ticket(1, core.StatusReady), ticket(2, core.StatusReady),
		ticket(3, core.StatusReady), ticket(4, core.StatusReady),
	})

	h.tick() // fires 1, 2 — both now active runs, board still says ready
	h.clock = h.clock.Add(time.Second)
	h.tick() // must fire nothing: two runs are in flight

	if got, want := h.disp.firedTickets(), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — active runs must fill the cap", got, want)
	}
}

// TestInProgressTicketsCountTowardTheCap: work a human dispatched, or an
// earlier process started, occupies a slot just the same.
func TestInProgressTicketsCountTowardTheCap(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2})
	h.boards.set("acme", []core.BoardTicket{
		ticket(1, core.StatusInProgress),
		ticket(2, core.StatusInProgress),
		ticket(3, core.StatusReady),
	})

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("fired %v, want none — the cap is already full of in_progress work", got)
	}
}

// TestSettlesRunWhenBranchAppears: once the ticket leaves `ready`, the run has
// done its job and must free its slot.
func TestSettlesRunWhenBranchAppears(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 1})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady), ticket(2, core.StatusReady)})

	h.tick() // fires 1

	// The agent pushed its branch: ticket 1 is now in_progress, and the board
	// shows it. The run should settle, but 1 still occupies the cap as
	// in_progress work.
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusInProgress), ticket(2, core.StatusReady)})
	h.clock = h.clock.Add(time.Minute)
	h.tick()

	stored := h.runs.get(1)
	if stored.active {
		t.Error("run stayed active after its ticket left the ready queue")
	}
	if stored.outcome != Observed {
		t.Errorf("run outcome = %q, want %q", stored.outcome, Observed)
	}
	if got, want := h.disp.firedTickets(), []int{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — ticket 1 still fills the cap as in_progress", got, want)
	}

	// Ticket 1 merges. The slot frees and 2 goes.
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusDone), ticket(2, core.StatusReady)})
	h.clock = h.clock.Add(time.Minute)
	h.tick()

	if got, want := h.disp.firedTickets(), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — the freed slot should start the next ticket", got, want)
	}
}

// TestBlockedTicketsAreNeverFired: the dependency graph is the derive engine's
// output, and the scheduler must respect it rather than re-deriving it.
func TestBlockedTicketsAreNeverFired(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 5})
	h.boards.set("acme", []core.BoardTicket{
		ticket(1, core.StatusBlocked),
		ticket(2, core.StatusNeedsAttention),
		ticket(3, core.StatusInReview),
		ticket(4, core.StatusDone),
	})

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("fired %v, want none — only `ready` is dispatchable", got)
	}
}

// TestAutopilotOffMeansNothingHappens: the switch has to actually be a switch.
func TestAutopilotOffMeansNothingHappens(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 5})
	h.disp.autopilotOn = false
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("fired %v with autopilot off, want none", got)
	}
}

// TestUnreadableAutopilotMeansOff: a project with no .claude/autopilot.json
// must not be treated as opted in. Unreadable is never "go ahead".
func TestUnreadableAutopilotMeansOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 5})
	h.disp.autopilotOn, h.disp.autopilotErr = true, errBoom
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("fired %v when autopilot could not be read, want none", got)
	}
}

// TestSchedulerNeverMerges is the safety property ADR-007 keeps: starting work
// is recoverable, landing it on main is not. The scheduler dispatches only.
func TestSchedulerNeverMerges(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 5})
	h.boards.set("acme", []core.BoardTicket{
		ticket(1, core.StatusReady),
		ticket(2, core.StatusInReview), // has a PR waiting — tempting, still not ours
	})

	h.tick()

	if h.disp.merged {
		t.Fatal("the scheduler merged a pull request; it must only ever dispatch")
	}
}

// TestTimesOutARunWhoseBranchNeverAppears and retries it, up to MaxAttempts.
func TestTimesOutAndRetriesThenGivesUp(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 1, RunTimeout: 10 * time.Minute, MaxAttempts: 2})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick() // attempt 1
	if got, want := h.disp.firedTickets(), []int{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fired %v, want %v", got, want)
	}

	// Still ready, well past the timeout: the run is written off and retried.
	h.clock = h.clock.Add(11 * time.Minute)
	h.tick()
	if got, want := h.disp.firedTickets(), []int{1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fired %v, want a retry after the timeout (%v)", got, want)
	}
	if stored := h.runs.get(1); stored.outcome != TimedOut {
		t.Errorf("first run outcome = %q, want %q", stored.outcome, TimedOut)
	}

	// The retry also times out. MaxAttempts is 2, so the scheduler stops and
	// leaves the ticket for a human rather than looping forever.
	h.clock = h.clock.Add(11 * time.Minute)
	h.tick()
	h.clock = h.clock.Add(11 * time.Minute)
	h.tick()

	if got, want := h.disp.firedTickets(), []int{1, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — MaxAttempts must stop the retries", got, want)
	}
}

// TestFireFailureIsRecordedAndCountsAsAnAttempt: a permanently misconfigured
// project (no routine trigger, bad token) must stop retrying, not hammer.
func TestFireFailureIsRecordedAndCountsAsAnAttempt(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2, MaxAttempts: 2})
	h.disp.fireErr = errBoom
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()
	stored := h.runs.get(1)
	if stored.active {
		t.Error("a failed dispatch left its run active, which would block the ticket forever")
	}
	if stored.outcome != Failed {
		t.Errorf("run outcome = %q, want %q", stored.outcome, Failed)
	}

	h.clock = h.clock.Add(time.Second)
	h.tick() // attempt 2
	h.clock = h.clock.Add(time.Second)
	h.tick() // must stop

	if got := h.disp.firedTickets(); len(got) != 2 {
		t.Errorf("fired %d times, want 2 — a failing dispatch still burns attempts", len(got))
	}
}

// TestPublishesDispatchStarted so the UI updates the moment work begins,
// exactly as it does for a human-driven dispatch.
func TestPublishesDispatchStarted(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 1})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()

	events := h.events.all()
	if len(events) != 1 {
		t.Fatalf("published %d events, want 1: %+v", len(events), events)
	}
	if events[0].kind != EventDispatchStarted {
		t.Errorf("event kind = %q, want %q", events[0].kind, EventDispatchStarted)
	}
	if events[0].data["ticket_id"] != 1 || events[0].data["scheduled"] != true {
		t.Errorf("event data = %+v, want ticket 1 marked as scheduled", events[0].data)
	}
}

// TestFailedDispatchPublishesNothing: an event saying work started, when it
// did not, would light up the UI for a session that does not exist.
func TestFailedDispatchPublishesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 1})
	h.disp.fireErr = errBoom
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()

	if events := h.events.all(); len(events) != 0 {
		t.Errorf("published %+v for a failed dispatch, want nothing", events)
	}
}

// TestBoardFailureSkipsProjectWithoutDispatching: an unreadable checkout must
// never be read as "no work in progress, fire everything".
func TestBoardFailureSkipsProjectWithoutDispatching(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 5})
	h.boards.err = errBoom

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("fired %v despite an unreadable board, want none", got)
	}
}

// TestDisabledSchedulerNeverTicks: the default configuration must be inert.
func TestDisabledSchedulerNeverTicks(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{Interval: 0, MaxParallel: 5})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { h.sched.Run(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run with a zero interval did not return immediately")
	}
	if got := h.disp.firedTickets(); len(got) != 0 {
		t.Errorf("a disabled scheduler fired %v", got)
	}
}

// TestRunStopsOnContextCancel: no goroutine may outlive the server.
func TestRunStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{Interval: 10 * time.Millisecond, MaxParallel: 1})
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.sched.Run(ctx); close(done) }()

	// Let it tick at least once, then stop it.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}
	if got := h.disp.firedTickets(); len(got) == 0 {
		t.Error("the scheduler never ticked before being canceled")
	}
}

// TestUnrecordableRunDoesNotSpiral is the nastiest failure mode: the routine
// was fired, but persisting the run failed. The scheduler now has no memory of
// work that is genuinely running. It must not compound that by treating the
// ticket as never-dispatched and firing it again every tick — the parallelism
// cap is the backstop that keeps a broken store from becoming a fork bomb.
func TestUnrecordableRunDoesNotSpiral(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 1})
	h.runs.startErr = errBoom
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady)})

	h.tick()
	// The agent really did start, so the branch shows up on the next board.
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusInProgress)})
	h.clock = h.clock.Add(time.Second)
	h.tick()
	h.clock = h.clock.Add(time.Second)
	h.tick()

	if got, want := h.disp.firedTickets(), []int{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — once the branch is visible the board alone must stop the re-fire", got, want)
	}
}

// TestSeededInFlightRunSurvivesRestart: runs outlive the process, so a
// scheduler starting up mid-flight must respect a run it never fired itself.
func TestSeededInFlightRunSurvivesRestart(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2, RunTimeout: time.Hour})
	// As if a previous process fired ticket 1 a minute before the restart.
	h.runs.seed("acme", 1, 1, t0.Add(-time.Minute), true, "")
	h.boards.set("acme", []core.BoardTicket{ticket(1, core.StatusReady), ticket(2, core.StatusReady)})

	h.tick()

	if got, want := h.disp.firedTickets(), []int{2}; !reflect.DeepEqual(got, want) {
		t.Errorf("fired %v, want %v — ticket 1's inherited run must still block it", got, want)
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()
	var zero Config
	if zero.maxParallel() != DefaultMaxParallel {
		t.Errorf("maxParallel() = %d, want %d", zero.maxParallel(), DefaultMaxParallel)
	}
	if zero.runTimeout() != DefaultRunTimeout {
		t.Errorf("runTimeout() = %s, want %s", zero.runTimeout(), DefaultRunTimeout)
	}
	if zero.maxAttempts() != DefaultMaxAttempts {
		t.Errorf("maxAttempts() = %d, want %d", zero.maxAttempts(), DefaultMaxAttempts)
	}
}

// TestScheduledDispatchCarriesTheAgentBriefing: a ticket the scheduler starts
// must arrive at the routine with the same instructions a human-dispatched one
// would. Two dispatch paths producing differently-instructed agents for the
// same ticket would be a silent behaviour split.
func TestScheduledDispatchCarriesTheAgentBriefing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2})
	h.briefs.set("backend", core.Briefing{
		AgentName: "Backend specialist",
		Prompt:    "Follow the repo's Go conventions.",
		Skills:    []string{"go", "sqlite"},
	})
	h.boards.set("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "T", Role: "backend"}, Status: core.StatusReady},
	})

	h.tick()

	got := h.disp.briefings()
	if len(got) != 1 {
		t.Fatalf("recorded %d briefings, want 1", len(got))
	}
	if got[0].AgentName != "Backend specialist" || got[0].Prompt == "" {
		t.Errorf("briefing = %+v, want the configured agent's name and prompt", got[0])
	}
	if len(got[0].Skills) != 2 {
		t.Errorf("briefing skills = %v, want the configured two", got[0].Skills)
	}
	if want := "acme/backend"; len(h.briefs.askedFor) == 0 || h.briefs.askedFor[0] != want {
		t.Errorf("asked for briefing %v, want %q — the ticket's own role", h.briefs.askedFor, want)
	}
}

// TestUnstaffedRoleDispatchesWithAnEmptyBriefing: a project that configured no
// agents must dispatch exactly as it did before agents existed.
func TestUnstaffedRoleDispatchesWithAnEmptyBriefing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, Config{MaxParallel: 2})
	h.boards.set("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "T", Role: "qa"}, Status: core.StatusReady},
	})

	h.tick()

	if got := h.disp.firedTickets(); len(got) != 1 {
		t.Fatalf("fired %v, want the ticket to dispatch despite having no agent", got)
	}
	if got := h.disp.briefings(); len(got) != 1 || !got[0].Empty() {
		t.Errorf("briefing = %+v, want an empty one for an unstaffed role", got)
	}
}
