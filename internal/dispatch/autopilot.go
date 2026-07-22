package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Markuysa/flightdeck/internal/core"
)

// autopilotRelPath is .claude/autopilot.json's location relative to a
// project's RepoPath — the file next-ticket/execute-ticket read to decide
// whether unattended work may start (see .claude/autopilot.json's own
// "note" field).
const autopilotRelPath = ".claude/autopilot.json"

// ErrAutopilotUnavailable wraps any failure reading or writing a project's
// autopilot.json: the file is missing, unreadable, not valid JSON, or its
// "enabled" field is missing or not a bool.
var ErrAutopilotUnavailable = errors.New("dispatch: autopilot file unavailable")

// autopilotPath returns the .claude/autopilot.json path inside repoPath.
func autopilotPath(repoPath string) string {
	return filepath.Join(repoPath, autopilotRelPath)
}

// readAutopilotFile reads and parses repoPath's autopilot.json into a
// map[string]json.RawMessage. Decoding into raw messages, rather than a
// fixed struct, is what lets SetAutopilot round-trip fields this package
// does not know about (or a future field added to the file) unchanged: only
// the "enabled" key is ever replaced; every other key's raw bytes pass
// through untouched.
func readAutopilotFile(repoPath string) (map[string]json.RawMessage, error) {
	path := autopilotPath(repoPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrAutopilotUnavailable, path, err)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: parsing %s: %w", ErrAutopilotUnavailable, path, err)
	}
	return cfg, nil
}

// decodeEnabled extracts and decodes cfg's "enabled" field.
func decodeEnabled(cfg map[string]json.RawMessage, path string) (bool, error) {
	raw, ok := cfg["enabled"]
	if !ok {
		return false, fmt.Errorf("%w: %s has no \"enabled\" field", ErrAutopilotUnavailable, path)
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("%w: %s \"enabled\" field is not a bool: %w", ErrAutopilotUnavailable, path, err)
	}
	return enabled, nil
}

// Autopilot implements core.Dispatcher.Autopilot: it reads p.RepoPath's
// local .claude/autopilot.json and returns its "enabled" field.
//
// This is a local-checkout operation only. Reading the file from a project's
// remote (e.g. its default branch on GitHub, for a project FlightDeck has no
// local clone of) is out of scope here — noted as future work, not
// implemented, since every registered project in v1 has a local RepoPath
// (ARCHITECTURE.md's Project.RepoPath: "local checkout FlightDeck reads").
func (c *Client) Autopilot(_ context.Context, p core.Project) (bool, error) {
	cfg, err := readAutopilotFile(p.RepoPath)
	if err != nil {
		return false, err
	}
	return decodeEnabled(cfg, autopilotPath(p.RepoPath))
}

// SetAutopilot implements core.Dispatcher.SetAutopilot: it flips the
// "enabled" field in p.RepoPath's local .claude/autopilot.json to on,
// preserving every other field in the file byte-for-byte (see
// readAutopilotFile). Like Autopilot, this is the local-checkout path only;
// writing via a project's remote is future work.
func (c *Client) SetAutopilot(_ context.Context, p core.Project, on bool) error {
	path := autopilotPath(p.RepoPath)
	cfg, err := readAutopilotFile(p.RepoPath)
	if err != nil {
		return err
	}

	raw, err := json.Marshal(on)
	if err != nil {
		return fmt.Errorf("%w: encoding enabled value: %w", ErrAutopilotUnavailable, err)
	}
	cfg["enabled"] = raw

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: encoding %s: %w", ErrAutopilotUnavailable, path, err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("%w: writing %s: %w", ErrAutopilotUnavailable, path, err)
	}
	return nil
}
