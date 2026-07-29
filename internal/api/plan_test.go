package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/plan"
)

// fakePlanner returns a canned plan and records what it was asked.
type fakePlanner struct {
	mu       sync.Mutex
	result   plan.Plan
	err      error
	goals    []string
	existing [][]core.Ticket
}

func (f *fakePlanner) Propose(_ context.Context, goal string, existing []core.Ticket) (plan.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.goals = append(f.goals, goal)
	f.existing = append(f.existing, existing)
	return f.result, f.err
}

func samplePlan() plan.Plan {
	return plan.Plan{
		Summary: "Split into a foundation and a feature.",
		Tickets: []plan.Ticket{
			{Number: 1, Title: "Auth scaffold", Role: "backend", Body: "b", Acceptance: []string{"tests pass"}},
			{Number: 2, Title: "Invoice API", Role: "backend", Depends: []int{1}, Body: "b"},
		},
	}
}

// TestProposePlanWritesNothing is the safety property the two-step design
// exists for: a proposal must never touch the repository. A planner that could
// write docs/tickets/ directly would be able to redirect every agent the
// scheduler subsequently dispatches.
func TestProposePlanWritesNothing(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ts := newTestServer()
	ts.planner.result = samplePlan()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: repo}))

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan", PlanRequest{Goal: "Add billing"}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("propose = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[plan.Plan](t, rec)
	if len(got.Tickets) != 2 {
		t.Errorf("proposed %d tickets, want 2", len(got.Tickets))
	}

	if _, err := os.Stat(filepath.Join(repo, "docs", "tickets")); !os.IsNotExist(err) {
		t.Error("proposing a plan created files in the repository; it must only propose")
	}
}

// TestProposePlanPassesTheExistingQueue: a plan that doesn't know what is
// already queued proposes duplicate work.
func TestProposePlanPassesTheExistingQueue(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	ts.planner.result = samplePlan()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: t.TempDir()}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 7, Title: "Already queued"}, Status: core.StatusReady},
	})

	doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan", PlanRequest{Goal: "Add billing"}, ts.token)

	if len(ts.planner.existing) != 1 || len(ts.planner.existing[0]) != 1 {
		t.Fatalf("planner saw existing = %v, want the one queued ticket", ts.planner.existing)
	}
	if ts.planner.existing[0][0].Title != "Already queued" {
		t.Errorf("planner saw %q, want the queued ticket's title", ts.planner.existing[0][0].Title)
	}
}

// TestProposePlanWithoutAKeyIs501: a setup gap, not an upstream failure — the
// two send an operator to entirely different places.
func TestProposePlanWithoutAKeyIs501(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	ts.planner.err = plan.ErrNoAPIKey
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: t.TempDir()}))

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan", PlanRequest{Goal: "x"}, ts.token)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("propose with no API key = %d, want 501: %s", rec.Code, rec.Body.String())
	}
}

// TestProposePlanWithABadGraphIs422: the model answered; we refuse its graph.
// Nothing upstream failed, so 502 would be the wrong answer.
func TestProposePlanWithABadGraphIs422(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	ts.planner.err = plan.ErrInvalidPlan
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: t.TempDir()}))

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan", PlanRequest{Goal: "x"}, ts.token)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("propose of an invalid plan = %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func TestApplyPlanWritesTickets(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ts := newTestServer()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: repo}))

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan/apply",
		ApplyPlanRequest{Plan: samplePlan()}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	applied := decodeJSON[plan.Applied](t, rec)
	if len(applied.Files) != 2 {
		t.Fatalf("applied %d files, want 2", len(applied.Files))
	}
	for _, rel := range applied.Files {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Errorf("reported file %s missing: %v", rel, err)
		}
	}
}

// TestApplyPlanRevalidates: the plan round-trips through the browser, so an
// edited one must be checked exactly as hard as a generated one. Trusting the
// client here would let a hand-edited cycle reach the queue.
func TestApplyPlanRevalidates(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ts := newTestServer()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: repo}))

	cyclic := plan.Plan{Tickets: []plan.Ticket{
		{Number: 1, Title: "A", Role: "backend", Depends: []int{2}},
		{Number: 2, Title: "B", Role: "backend", Depends: []int{1}},
	}}
	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/plan/apply",
		ApplyPlanRequest{Plan: cyclic}, ts.token)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("apply of a client-edited cyclic plan = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cycle") {
		t.Errorf("body = %s, want it to name the cycle", rec.Body.String())
	}
	entries, _ := os.ReadDir(filepath.Join(repo, "docs", "tickets"))
	if len(entries) != 0 {
		t.Errorf("a rejected plan wrote %d files, want none", len(entries))
	}
}

func TestPlanRoutesRequireAuth(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: t.TempDir()}))

	for _, path := range []string{"/api/projects/acme/plan", "/api/projects/acme/plan/apply"} {
		rec := doRequest(t, ts.srv.Handler(), http.MethodPost, path, PlanRequest{Goal: "x"}, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated POST %s = %d, want 401", path, rec.Code)
		}
	}
}
