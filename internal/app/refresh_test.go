package app

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// runGit runs git against dir, failing the test on error. It mutates a
// fixture repo *after* git.NewFixtureRepo built it, to simulate an agent's
// work happening between two polls.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // fixed binary, fixed test args
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// newTestRegistry opens a temp-file registry.Store, closed automatically via
// t.Cleanup.
func newTestRegistry(t *testing.T) *registry.Store {
	t.Helper()
	store, err := registry.Open(filepath.Join(t.TempDir(), "flightdeck.db"))
	if err != nil {
		t.Fatalf("registry.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestRefresherPollOnce_BaselineThenChangePublishesBoardChanged is ticket
// 018's deterministic test: a fixture project's first poll only establishes
// a baseline (no event), then flipping a ticket's derived status (giving it
// a branch, with no commit needed — Repo.Branches lists it either way) makes
// the next poll publish board.changed to a subscriber. Fully offline: the
// fixture repo has no GitHub remote, so api.NewGitHubSource never reaches
// the network (see gitHubSource.openPRs).
func TestRefresherPollOnce_BaselineThenChangePublishesBoardChanged(t *testing.T) {
	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		nil, // no branches yet: ticket 2 starts out "ready" (no branch)
	)

	store := newTestRegistry(t)
	ctx := context.Background()
	project := core.Project{ID: "fixture", Name: "Fixture", RepoPath: repo.Path}
	if err := store.Add(ctx, project); err != nil {
		t.Fatalf("registering fixture project: %v", err)
	}

	broker := api.NewBroker()
	ch, cancel := broker.Subscribe()
	defer cancel()

	refresher := NewRefresher(broker, api.NewGitHubSource(store), store, time.Minute)

	// First poll: baseline only.
	refresher.pollOnce(ctx)
	select {
	case ev := <-ch:
		t.Fatalf("baseline poll published an event, want none: %+v", ev)
	default:
	}

	// Mutate: a claude/002-fixture branch now exists for ticket 2 (still
	// "todo" on it, same as main) — derive.Derive reads that as in_progress
	// instead of ready (mirrors TestSmoke_BoardOverHTTP's ticket 4 case), so
	// the board's structural fingerprint changes.
	runGit(t, repo.Path, "branch", "claude/002-fixture")

	refresher.pollOnce(ctx)
	select {
	case ev := <-ch:
		if ev.Kind != api.EventBoardChanged {
			t.Fatalf("event kind = %q, want %q", ev.Kind, api.EventBoardChanged)
		}
		data, ok := ev.Data.(map[string]any)
		if !ok {
			t.Fatalf("event data type = %T, want map[string]any", ev.Data)
		}
		if data["project_id"] != project.ID {
			t.Fatalf("event project_id = %v, want %q", data["project_id"], project.ID)
		}
	default:
		t.Fatal("poll after mutation published no event, want board.changed")
	}

	// Steady state: polling again with nothing changed publishes nothing.
	refresher.pollOnce(ctx)
	select {
	case ev := <-ch:
		t.Fatalf("unchanged poll published an event, want none: %+v", ev)
	default:
	}
}

// TestRefresherPollOnce_SkipsProjectErrorWithoutStoppingOthers proves the
// resilience acceptance criterion: a project whose board cannot be derived
// (a nonexistent repo path) is logged and skipped, never crashing the loop
// or blocking another, healthy project's poll in the same tick.
func TestRefresherPollOnce_SkipsProjectErrorWithoutStoppingOthers(t *testing.T) {
	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{{ID: 1, Title: "Solo", Role: "backend", Status: "done"}},
		nil,
	)

	store := newTestRegistry(t)
	ctx := context.Background()
	broken := core.Project{ID: "broken", Name: "Broken", RepoPath: filepath.Join(t.TempDir(), "does-not-exist")}
	healthy := core.Project{ID: "healthy", Name: "Healthy", RepoPath: repo.Path}
	if err := store.Add(ctx, broken); err != nil {
		t.Fatalf("registering broken project: %v", err)
	}
	if err := store.Add(ctx, healthy); err != nil {
		t.Fatalf("registering healthy project: %v", err)
	}

	broker := api.NewBroker()
	refresher := NewRefresher(broker, api.NewGitHubSource(store), store, time.Minute)

	// Two polls: the broken project errors both times (skipped, not
	// crashed); the healthy project gets a real baseline on the first and
	// nothing changed on the second, so pollOnce must return normally both
	// times with the healthy project's baseline recorded.
	refresher.pollOnce(ctx)
	refresher.pollOnce(ctx)

	if _, ok := refresher.baselines[healthy.ID]; !ok {
		t.Fatal("healthy project has no baseline after two polls")
	}
	if _, ok := refresher.baselines[broken.ID]; ok {
		t.Fatal("broken project unexpectedly has a baseline (BoardTickets should have errored)")
	}
}

// TestApp_RunStopsRefresherOnContextCancel wires a real App with the
// refresher enabled (RefreshInterval > 0, unlike smoke_test.go's Config{}
// which leaves it at its disabled zero value) and proves Run's goroutine for
// it actually stops when ctx is canceled — the "no goroutine leak"
// acceptance criterion — rather than only asserting on pollOnce in
// isolation.
func TestApp_RunStopsRefresherOnContextCancel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	a, err := New(Config{Token: "smoke-test-token", Addr: ":0", DBPath: dbPath, RefreshInterval: time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error after cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx was canceled — refresher goroutine leak")
	}
}

func TestParseRefreshInterval(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{name: "empty defaults", raw: "", want: defaultRefreshInterval},
		{name: "off disables", raw: "off", want: 0},
		{name: "OFF case-insensitive", raw: "OFF", want: 0},
		{name: "zero disables", raw: "0", want: 0},
		{name: "explicit duration", raw: "30s", want: 30 * time.Second},
		{name: "negative disables", raw: "-5s", want: 0},
		{name: "garbage errors", raw: "not-a-duration", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRefreshInterval(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRefreshInterval(%q) = %v, nil; want an error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRefreshInterval(%q) unexpected error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("parseRefreshInterval(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
