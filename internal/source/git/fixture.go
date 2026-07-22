package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FixtureTicket describes one docs/tickets file to write in a fixture repo
// built by NewFixtureRepo, or one ticket's contents to override on a
// specific branch via FixtureBranch.Tickets.
type FixtureTicket struct {
	ID      int
	Title   string
	Role    string
	Depends []int
	Status  string // todo | done | needs-attention
	Body    string // optional; a placeholder body is used when empty
}

// FixtureBranch describes one branch to create in a fixture repo, branched
// from main.
type FixtureBranch struct {
	Name string

	// Tickets rewrites the listed tickets' files on this branch and commits
	// the change; list only the tickets whose content differs from main
	// (typically to flip Status, simulating an agent's work). Every field is
	// written verbatim, including Title and Role, so keep them identical to
	// the corresponding main-branch FixtureTicket unless the scenario is
	// deliberately about a diverging title. Leave Tickets empty for a branch
	// that starts out identical to main (e.g. "in progress, untouched").
	Tickets []FixtureTicket

	// Merged, when true, merges this branch into main (--no-ff) right after
	// creating it, so IsMergedToMain reports true for Name.
	Merged bool
}

// NewFixtureRepo builds a temporary git repository — in a directory from
// t.TempDir(), cleaned up automatically — with the given tickets committed
// to docs/tickets on the default branch ("main"), then creates each branch
// in order, applying its ticket overrides and merging it back to main when
// requested.
//
// It shells out to the git binary and sets user.email/user.name locally in
// the new repo, so it needs no global git identity and works on any
// machine. Every call gets its own temp directory and touches no shared or
// global git state, so it is safe for t.Parallel().
//
// Tickets 005 (derive) and 014 (qa) build their test fixtures on this
// helper; keep this signature — and the returned *Repo's behavior — stable.
func NewFixtureRepo(t *testing.T, tickets []FixtureTicket, branches []FixtureBranch) *Repo {
	t.Helper()
	dir := t.TempDir()

	runFixtureGit(t, dir, "init", "-q")
	runFixtureGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	runFixtureGit(t, dir, "config", "user.email", "flightdeck-fixture@example.com")
	runFixtureGit(t, dir, "config", "user.name", "FlightDeck Fixture")
	runFixtureGit(t, dir, "config", "commit.gpgsign", "false")

	for _, ft := range tickets {
		writeFixtureTicket(t, dir, ft)
	}
	runFixtureGit(t, dir, "add", "-A")
	runFixtureGit(t, dir, "commit", "-q", "-m", "fixture: initial tickets")

	for _, b := range branches {
		runFixtureGit(t, dir, "checkout", "-q", "main")
		runFixtureGit(t, dir, "checkout", "-q", "-b", b.Name)
		if len(b.Tickets) > 0 {
			for _, ft := range b.Tickets {
				writeFixtureTicket(t, dir, ft)
			}
			runFixtureGit(t, dir, "add", "-A")
			runFixtureGit(t, dir, "commit", "-q", "-m", fmt.Sprintf("fixture: %s", b.Name))
		}
		if b.Merged {
			runFixtureGit(t, dir, "checkout", "-q", "main")
			runFixtureGit(t, dir, "merge", "-q", "--no-ff", "-m", fmt.Sprintf("fixture: merge %s", b.Name), b.Name)
		}
	}
	runFixtureGit(t, dir, "checkout", "-q", "main")

	return &Repo{Path: dir, Main: "main"}
}

// fixtureTicketFilename derives a stable filename from a ticket id alone,
// deliberately ignoring Title — so overriding a ticket's Title on a branch
// (via FixtureBranch.Tickets) still overwrites the same file that was
// written on main, instead of leaving both versions on disk.
func fixtureTicketFilename(id int) string {
	return fmt.Sprintf("%03d-fixture.md", id)
}

func writeFixtureTicket(t *testing.T, repoDir string, ft FixtureTicket) {
	t.Helper()
	body := ft.Body
	if body == "" {
		body = "Fixture ticket body."
	}
	depends := make([]string, len(ft.Depends))
	for i, d := range ft.Depends {
		depends[i] = fmt.Sprintf("%d", d)
	}
	content := fmt.Sprintf(
		"---\nid: %d\ntitle: %s\nrole: %s\ndepends: [%s]\nstatus: %s\n---\n%s\n",
		ft.ID, ft.Title, ft.Role, strings.Join(depends, ", "), ft.Status, body,
	)

	path := filepath.Join(repoDir, "docs", "tickets", fixtureTicketFilename(ft.ID))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating docs/tickets in fixture repo: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // fixture file, not sensitive
		t.Fatalf("writing fixture ticket %d: %v", ft.ID, err)
	}
}

func runFixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // fixed binary, args built from constants and test input
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
