package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

// writeAutopilotFixture writes content as repoPath's .claude/autopilot.json
// and returns its path.
func writeAutopilotFixture(t *testing.T, repoPath, content string) string {
	t.Helper()
	dir := filepath.Join(repoPath, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "autopilot.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestAutopilotReadsEnabledField(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeAutopilotFixture(t, repo, `{"enabled": true, "maxInFlight": 1, "note": "kill switch"}`)
	c := New("rt", "gh")

	on, err := c.Autopilot(context.Background(), core.Project{RepoPath: repo})
	if err != nil {
		t.Fatalf("Autopilot: %v", err)
	}
	if !on {
		t.Errorf("Autopilot() = false, want true")
	}
}

// TestSetAutopilotPreservesOtherFields is the acceptance-criterion test:
// flipping enabled to false on a file that also carries maxInFlight and
// note must leave those two fields byte-for-byte unchanged.
func TestSetAutopilotPreservesOtherFields(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path := writeAutopilotFixture(t, repo,
		`{"enabled": true, "maxInFlight": 3, "note": "kill switch for unattended execution"}`)
	c := New("rt", "gh")
	p := core.Project{RepoPath: repo}

	if err := c.SetAutopilot(context.Background(), p, false); err != nil {
		t.Fatalf("SetAutopilot: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["enabled"] != false {
		t.Errorf("enabled = %v, want false", got["enabled"])
	}
	if got["maxInFlight"] != float64(3) {
		t.Errorf("maxInFlight = %v, want 3 (unchanged)", got["maxInFlight"])
	}
	if got["note"] != "kill switch for unattended execution" {
		t.Errorf("note = %v, want unchanged", got["note"])
	}

	// Reopening through Autopilot must reflect the flip too.
	on, err := c.Autopilot(context.Background(), p)
	if err != nil {
		t.Fatalf("Autopilot: %v", err)
	}
	if on {
		t.Errorf("Autopilot() = true after SetAutopilot(false), want false")
	}
}

func TestSetAutopilotFlipsOnAndBack(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeAutopilotFixture(t, repo, `{"enabled": false, "maxInFlight": 2, "note": "n"}`)
	c := New("rt", "gh")
	p := core.Project{RepoPath: repo}

	if err := c.SetAutopilot(context.Background(), p, true); err != nil {
		t.Fatalf("SetAutopilot(true): %v", err)
	}
	on, err := c.Autopilot(context.Background(), p)
	if err != nil {
		t.Fatalf("Autopilot: %v", err)
	}
	if !on {
		t.Errorf("Autopilot() = false after SetAutopilot(true), want true")
	}
}

func TestAutopilotMissingFileIsTypedError(t *testing.T) {
	t.Parallel()
	c := New("rt", "gh")
	p := core.Project{RepoPath: t.TempDir()}

	_, err := c.Autopilot(context.Background(), p)
	if err == nil {
		t.Fatal("Autopilot() error = nil, want ErrAutopilotUnavailable")
	}
	if !errors.Is(err, ErrAutopilotUnavailable) {
		t.Errorf("Autopilot() error = %v, want it to wrap ErrAutopilotUnavailable", err)
	}
}

func TestAutopilotMalformedFileIsTypedError(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeAutopilotFixture(t, repo, `not json`)
	c := New("rt", "gh")

	_, err := c.Autopilot(context.Background(), core.Project{RepoPath: repo})
	if !errors.Is(err, ErrAutopilotUnavailable) {
		t.Errorf("Autopilot() error = %v, want it to wrap ErrAutopilotUnavailable", err)
	}
}

func TestAutopilotMissingEnabledFieldIsTypedError(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeAutopilotFixture(t, repo, `{"maxInFlight": 1, "note": "no enabled key"}`)
	c := New("rt", "gh")

	_, err := c.Autopilot(context.Background(), core.Project{RepoPath: repo})
	if !errors.Is(err, ErrAutopilotUnavailable) {
		t.Errorf("Autopilot() error = %v, want it to wrap ErrAutopilotUnavailable", err)
	}
}
