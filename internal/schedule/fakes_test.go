package schedule

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

// fakeRunStore is an in-memory RunStore with the same semantics as the SQLite
// one: StartRun assigns ids, Settle is a no-op on an already-settled run.
type fakeRunStore struct {
	mu       sync.Mutex
	nextID   int64
	byID     map[int64]*storedRun
	startErr error
}

type storedRun struct {
	Run
	projectID string
	active    bool
	outcome   Outcome
	detail    string
}

func newFakeRunStore() *fakeRunStore {
	return &fakeRunStore{byID: map[int64]*storedRun{}}
}

func (f *fakeRunStore) ActiveRuns(_ context.Context, projectID string) ([]Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	runs := []Run{}
	for _, r := range f.byID {
		if r.projectID == projectID && r.active {
			runs = append(runs, r.Run)
		}
	}
	// Deterministic order by id, matching the store's "oldest first".
	for i := range runs {
		for j := i + 1; j < len(runs); j++ {
			if runs[j].ID < runs[i].ID {
				runs[i], runs[j] = runs[j], runs[i]
			}
		}
	}
	return runs, nil
}

func (f *fakeRunStore) LatestRun(_ context.Context, projectID string, ticketID int) (Run, bool, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *storedRun
	for _, r := range f.byID {
		if r.projectID != projectID || r.TicketID != ticketID {
			continue
		}
		if latest == nil || r.ID > latest.ID {
			latest = r
		}
	}
	if latest == nil {
		return Run{}, false, false, nil
	}
	return latest.Run, latest.active, true, nil
}

func (f *fakeRunStore) StartRun(_ context.Context, projectID string, ticketID, attempt int, sessionURL string, at time.Time) (Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return Run{}, f.startErr
	}
	f.nextID++
	run := Run{ID: f.nextID, TicketID: ticketID, Attempt: attempt, SessionURL: sessionURL, StartedAt: at}
	f.byID[run.ID] = &storedRun{Run: run, projectID: projectID, active: true}
	return run, nil
}

func (f *fakeRunStore) Settle(_ context.Context, runID int64, outcome Outcome, detail string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[runID]
	if !ok || !r.active {
		return nil
	}
	r.active, r.outcome, r.detail = false, outcome, detail
	return nil
}

// seed inserts an already-existing run, for tests that start mid-flight.
func (f *fakeRunStore) seed(projectID string, ticketID, attempt int, startedAt time.Time, active bool, outcome Outcome) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	run := Run{ID: f.nextID, TicketID: ticketID, Attempt: attempt, StartedAt: startedAt}
	f.byID[run.ID] = &storedRun{Run: run, projectID: projectID, active: active, outcome: outcome}
	return run.ID
}

func (f *fakeRunStore) get(id int64) storedRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.byID[id]
}

// fakeDispatcher records every Fire and answers Autopilot from a flag.
type fakeDispatcher struct {
	mu           sync.Mutex
	fired        []int
	briefs       []core.Briefing
	merged       bool
	autopilotOn  bool
	autopilotErr error
	fireErr      error
}

func (d *fakeDispatcher) Fire(_ context.Context, _ core.Project, ticketID int, brief core.Briefing) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fired = append(d.fired, ticketID)
	d.briefs = append(d.briefs, brief)
	if d.fireErr != nil {
		return "", d.fireErr
	}
	return "https://claude.ai/code/s/x", nil
}

func (d *fakeDispatcher) Autopilot(context.Context, core.Project) (bool, error) {
	return d.autopilotOn, d.autopilotErr
}

func (d *fakeDispatcher) SetAutopilot(context.Context, core.Project, bool) error { return nil }

// ApproveMerge fails the test if it is ever reached: the scheduler must never
// merge. See TestSchedulerNeverMerges.
func (d *fakeDispatcher) ApproveMerge(context.Context, core.Project, int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.merged = true
	return nil
}

func (d *fakeDispatcher) briefings() []core.Briefing {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]core.Briefing(nil), d.briefs...)
}

func (d *fakeDispatcher) firedTickets() []int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]int(nil), d.fired...)
}

type fakeFactory struct{ d *fakeDispatcher }

func (f fakeFactory) Dispatcher(context.Context, core.Project) (core.Dispatcher, error) {
	return f.d, nil
}

type fakeProjects struct{ list []core.Project }

func (f fakeProjects) List(context.Context) ([]core.Project, error) { return f.list, nil }

type fakeBoards struct {
	mu      sync.Mutex
	tickets map[string][]core.BoardTicket
	err     error
}

func (f *fakeBoards) BoardTickets(_ context.Context, p core.Project) ([]core.BoardTicket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.tickets[p.ID], nil
}

func (f *fakeBoards) set(projectID string, tickets []core.BoardTicket) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tickets == nil {
		f.tickets = map[string][]core.BoardTicket{}
	}
	f.tickets[projectID] = tickets
}

type recordedEvent struct {
	kind string
	data map[string]any
}

type fakePublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (p *fakePublisher) Publish(kind string, data map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, recordedEvent{kind, data})
}

func (p *fakePublisher) all() []recordedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]recordedEvent(nil), p.events...)
}

var errBoom = errors.New("boom")

// ticket builds a board ticket in one status.
func ticket(id int, status core.DerivedStatus) core.BoardTicket {
	return core.BoardTicket{Ticket: core.Ticket{ID: id, Title: "T"}, Status: status}
}

// fakeBriefings records which (project, role) pairs the scheduler asked about
// and answers with a canned briefing per role.
type fakeBriefings struct {
	mu       sync.Mutex
	byRole   map[string]core.Briefing
	askedFor []string
}

func (f *fakeBriefings) Briefing(_ context.Context, projectID, role string) core.Briefing {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.askedFor = append(f.askedFor, projectID+"/"+role)
	return f.byRole[role]
}

func (f *fakeBriefings) set(role string, b core.Briefing) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byRole == nil {
		f.byRole = map[string]core.Briefing{}
	}
	f.byRole[role] = b
}
