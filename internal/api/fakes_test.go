package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// fakeRegistry is an in-memory ProjectRegistry + SecretsReader for handler
// tests — no SQLite, no filesystem, matching the ticket's "handler tests
// use fake core interfaces; no network" acceptance criterion.
type fakeRegistry struct {
	mu       sync.Mutex
	projects map[string]core.Project
	secrets  map[string]registry.Secrets
	order    []string
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		projects: map[string]core.Project{},
		secrets:  map[string]registry.Secrets{},
	}
}

func (f *fakeRegistry) Add(_ context.Context, p core.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.projects[p.ID]; ok {
		return fmt.Errorf("project %q already exists", p.ID)
	}
	f.projects[p.ID] = p
	f.order = append(f.order, p.ID)
	return nil
}

func (f *fakeRegistry) List(_ context.Context) ([]core.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]core.Project, 0, len(f.order))
	for _, id := range f.order {
		out = append(out, f.projects[id])
	}
	return out, nil
}

func (f *fakeRegistry) Get(_ context.Context, id string) (core.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.projects[id]
	if !ok {
		return core.Project{}, core.ErrProjectNotFound
	}
	return p, nil
}

func (f *fakeRegistry) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.projects[id]; !ok {
		return core.ErrProjectNotFound
	}
	delete(f.projects, id)
	delete(f.secrets, id)
	for i, oid := range f.order {
		if oid == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeRegistry) Secrets(_ context.Context, id string) (registry.Secrets, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.projects[id]; !ok {
		return registry.Secrets{}, core.ErrProjectNotFound
	}
	return f.secrets[id], nil
}

func (f *fakeRegistry) SetSecrets(_ context.Context, id string, sec registry.Secrets) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secrets[id] = sec
	return nil
}

// fakeSource is an in-memory ProjectSource: tests preload each project's
// board (and, when needed, a branch's commit time) rather than deriving it
// from a real checkout.
type fakeSource struct {
	mu          sync.Mutex
	boards      map[string][]core.BoardTicket
	boardErr    map[string]error
	commitTimes map[string]time.Time
}

func newFakeSource() *fakeSource {
	return &fakeSource{
		boards:      map[string][]core.BoardTicket{},
		boardErr:    map[string]error{},
		commitTimes: map[string]time.Time{},
	}
}

func (f *fakeSource) setBoard(projectID string, tickets []core.BoardTicket) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.boards[projectID] = tickets
}

func (f *fakeSource) setBoardErr(projectID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.boardErr[projectID] = err
}

func (f *fakeSource) BoardTickets(_ context.Context, p core.Project) ([]core.BoardTicket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.boardErr[p.ID]; ok {
		return nil, err
	}
	return f.boards[p.ID], nil
}

func (f *fakeSource) setCommitTime(projectID, branch string, t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commitTimes[projectID+"/"+branch] = t
}

func (f *fakeSource) BranchCommitTime(_ context.Context, p core.Project, branch string) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.commitTimes[p.ID+"/"+branch]
	if !ok {
		return time.Time{}, fmt.Errorf("no commit time recorded for %s/%s", p.ID, branch)
	}
	return t, nil
}

// fakeDispatcher is an in-memory core.Dispatcher recording every call, so
// tests can assert exactly which ticket Fire targeted and exactly which PR
// number ApproveMerge targeted — no live routine or GitHub call.
type fakeDispatcher struct {
	mu              sync.Mutex
	firedTicketIDs  []int
	fireErr         error
	sessionURL      string
	autopilotOn     bool
	autopilotErr    error
	setAutopilotErr error
	approvedPRs     []int
	approveErr      error
}

var _ core.Dispatcher = (*fakeDispatcher)(nil)

func (d *fakeDispatcher) Fire(_ context.Context, _ core.Project, ticketID int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.firedTicketIDs = append(d.firedTicketIDs, ticketID)
	if d.fireErr != nil {
		return "", d.fireErr
	}
	return d.sessionURL, nil
}

func (d *fakeDispatcher) Autopilot(_ context.Context, _ core.Project) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.autopilotOn, d.autopilotErr
}

func (d *fakeDispatcher) SetAutopilot(_ context.Context, _ core.Project, on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.setAutopilotErr != nil {
		return d.setAutopilotErr
	}
	d.autopilotOn = on
	return nil
}

func (d *fakeDispatcher) ApproveMerge(_ context.Context, _ core.Project, prNumber int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.approvedPRs = append(d.approvedPRs, prNumber)
	return d.approveErr
}

// fakeDispatcherFactory hands out one fakeDispatcher per project ID,
// creating it lazily, so a test can fetch it back via forProject to set up
// expectations or assert on recorded calls.
type fakeDispatcherFactory struct {
	mu          sync.Mutex
	dispatchers map[string]*fakeDispatcher
	err         error
}

func newFakeDispatcherFactory() *fakeDispatcherFactory {
	return &fakeDispatcherFactory{dispatchers: map[string]*fakeDispatcher{}}
}

func (f *fakeDispatcherFactory) forProject(id string) *fakeDispatcher {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.dispatchers[id]
	if !ok {
		d = &fakeDispatcher{}
		f.dispatchers[id] = d
	}
	return d
}

func (f *fakeDispatcherFactory) Dispatcher(_ context.Context, p core.Project) (core.Dispatcher, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.forProject(p.ID), nil
}

// testServer bundles a Server with its fakes so a test can both drive HTTP
// requests through it and set up/assert on the fakes directly.
type testServer struct {
	srv        *Server
	registry   *fakeRegistry
	source     *fakeSource
	dispatcher *fakeDispatcherFactory
	token      string
}

const testToken = "test-flightdeck-token"

func newTestServer() *testServer {
	ts := &testServer{
		registry:   newFakeRegistry(),
		source:     newFakeSource(),
		dispatcher: newFakeDispatcherFactory(),
		token:      testToken,
	}
	ts.srv = NewServer(Config{
		Token:      ts.token,
		Registry:   ts.registry,
		Source:     ts.source,
		Dispatcher: ts.dispatcher,
	})
	return ts
}
