package e2e

import (
	"context"
	"sync"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
)

// fireCall and approveCall record exactly what handleDispatch/handleApprove
// asked the routine/GitHub to do — this suite's assertion surface for "Fire
// gets the right payload" and "approve merges only the named PR".
type fireCall struct {
	ProjectID string
	TicketID  int
}

type approveCall struct {
	ProjectID string
	PRNumber  int
}

// stubDispatcher is this suite's stubbed routine + GitHub-merge boundary
// (ticket 014's instructions): it never makes an HTTP call anywhere, it
// just records every Fire/ApproveMerge invocation and returns a canned
// session URL, so dispatch and approve can be asserted without a live
// routine or GitHub.
type stubDispatcher struct {
	mu           sync.Mutex
	fireCalls    []fireCall
	approveCalls []approveCall
	sessionURL   string
	fireErr      error
	approveErr   error
	autopilotOn  bool
}

var _ core.Dispatcher = (*stubDispatcher)(nil)

func newStubDispatcher() *stubDispatcher {
	return &stubDispatcher{sessionURL: "https://routines.example.com/sessions/e2e-fixture"}
}

func (d *stubDispatcher) Fire(_ context.Context, p core.Project, ticketID int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fireCalls = append(d.fireCalls, fireCall{ProjectID: p.ID, TicketID: ticketID})
	if d.fireErr != nil {
		return "", d.fireErr
	}
	return d.sessionURL, nil
}

func (d *stubDispatcher) Autopilot(_ context.Context, _ core.Project) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.autopilotOn, nil
}

func (d *stubDispatcher) SetAutopilot(_ context.Context, _ core.Project, on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.autopilotOn = on
	return nil
}

func (d *stubDispatcher) ApproveMerge(_ context.Context, p core.Project, prNumber int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.approveCalls = append(d.approveCalls, approveCall{ProjectID: p.ID, PRNumber: prNumber})
	return d.approveErr
}

// approvedPRNumbers returns the PR numbers ApproveMerge was called with, in
// call order.
func (d *stubDispatcher) approvedPRNumbers() []int {
	d.mu.Lock()
	defer d.mu.Unlock()
	nums := make([]int, len(d.approveCalls))
	for i, c := range d.approveCalls {
		nums[i] = c.PRNumber
	}
	return nums
}

// stubDispatcherFactory hands back the same stubDispatcher for every
// project — every test in this suite registers exactly one project, so a
// single shared recorder keeps assertions simple.
type stubDispatcherFactory struct {
	dispatcher *stubDispatcher
}

var _ api.DispatcherFactory = (*stubDispatcherFactory)(nil)

func newStubDispatcherFactory(d *stubDispatcher) *stubDispatcherFactory {
	return &stubDispatcherFactory{dispatcher: d}
}

func (f *stubDispatcherFactory) Dispatcher(_ context.Context, _ core.Project) (core.Dispatcher, error) {
	return f.dispatcher, nil
}
