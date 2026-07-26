package demo_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/demo"
	"github.com/Markuysa/flightdeck/internal/dispatch"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// noSecrets satisfies api.SecretsReader with no tokens set — sufficient
// here since the seeded project has no GitHub remote (Remote == ""), so
// api.NewGitHubSource never reads secrets for it.
type noSecrets struct{}

func (noSecrets) Secrets(context.Context, string) (registry.Secrets, error) {
	return registry.Secrets{}, nil
}

// TestSeed_RegistersProjectWithTicketsAcrossSeveralStatuses seeds into a
// temp registry, then feeds the registered project through the same
// real ProjectSource GET /api/projects/{id}/board uses — proving the
// seeded fixture repo actually derives into several distinct statuses, not
// just that files exist on disk. Fully offline: the registry is a temp
// SQLite file, the fixture repo is local-only, and the project has no
// GitHub remote.
func TestSeed_RegistersProjectWithTicketsAcrossSeveralStatuses(t *testing.T) {
	ctx := context.Background()
	store, err := registry.Open(filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("registry.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, err := demo.Seed(ctx, store)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if project.ID != demo.ProjectID {
		t.Errorf("project.ID = %q, want %q", project.ID, demo.ProjectID)
	}
	if project.Remote != "" {
		t.Errorf("project.Remote = %q, want \"\" (demo mode must never touch the network)", project.Remote)
	}

	registered, err := store.Get(ctx, demo.ProjectID)
	if err != nil {
		t.Fatalf("store.Get(%q): %v", demo.ProjectID, err)
	}
	if registered.RepoPath != project.RepoPath {
		t.Errorf("registered RepoPath = %q, want %q", registered.RepoPath, project.RepoPath)
	}

	source := api.NewGitHubSource(noSecrets{})
	board, err := source.BoardTickets(ctx, registered)
	if err != nil {
		t.Fatalf("BoardTickets: %v", err)
	}
	if len(board) == 0 {
		t.Fatal("BoardTickets returned no tickets")
	}

	statuses := make(map[core.DerivedStatus]bool)
	for _, bt := range board {
		statuses[bt.Status] = true
	}
	if len(statuses) < 4 {
		t.Fatalf("seeded board has %d distinct statuses (%v), want at least 4", len(statuses), statuses)
	}

	// Seeding again must not fail on the duplicate id (idempotent-ish).
	if _, err := demo.Seed(ctx, store); err != nil {
		t.Fatalf("Seed a second time: %v", err)
	}
}

// TestSeed_AutopilotRoundTrips proves the seeded repo carries a
// .claude/autopilot.json so the dispatcher can read and flip autopilot —
// i.e. the Fleet toggle (GET/PUT /api/projects/demo/autopilot) works against
// the demo project, not just a real repo. Local file ops only, no network.
func TestSeed_AutopilotRoundTrips(t *testing.T) {
	ctx := context.Background()
	store, err := registry.Open(filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("registry.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, err := demo.Seed(ctx, store)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// No tokens needed: Autopilot/SetAutopilot are local .claude/autopilot.json
	// operations against project.RepoPath.
	d := dispatch.New("", "")

	on, err := d.Autopilot(ctx, project)
	if err != nil {
		t.Fatalf("Autopilot (initial read): %v", err)
	}
	if on {
		t.Fatal("seeded autopilot = on, want off")
	}

	if err := d.SetAutopilot(ctx, project, true); err != nil {
		t.Fatalf("SetAutopilot(true): %v", err)
	}
	on, err = d.Autopilot(ctx, project)
	if err != nil {
		t.Fatalf("Autopilot (after flip): %v", err)
	}
	if !on {
		t.Fatal("autopilot after SetAutopilot(true) = off, want on")
	}
}
