// Package demo builds a seeded fixture project at runtime so a newcomer can
// explore FlightDeck with no real git repository: `flightdeck serve --demo`
// calls Seed once at startup (internal/app wires it in — ADR-004).
//
// This is deliberately NOT internal/source/git/fixture.go's NewFixtureRepo:
// that helper takes a *testing.T and can only run inside a test binary.
// Seed shells to the same git plumbing but as ordinary runtime code,
// returning errors instead of failing a test.
package demo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// ProjectID is the seeded demo project's stable id — also its URL slug
// (/p/demo) once registered.
const ProjectID = "demo"

const projectName = "Acme Ledger (demo)"

// Registry is the subset of registry.Store Seed needs. registry.Store
// satisfies it structurally — no wrapper needed, the same pattern
// internal/api's ProjectRegistry and SecretsReader already use.
type Registry interface {
	Add(ctx context.Context, p core.Project) error
	Remove(ctx context.Context, id string) error
}

// Seed builds a throwaway git repository under a fixed temp directory,
// seeded with tickets whose derived statuses span done, in_review,
// in_progress, needs_attention, ready and blocked, and registers it in reg
// as ProjectID — so `flightdeck serve --demo` has something to show with no
// real project configured. The registered core.Project has no GitHub remote
// (Remote is ""), so computing its board never touches the network.
//
// Seed is idempotent-ish: calling it again (e.g. every `serve --demo`
// restart) wipes and rebuilds the fixture directory and re-registers the
// project, rather than failing on the duplicate id.
func Seed(ctx context.Context, reg Registry) (core.Project, error) {
	repoDir := demoDir()
	if err := os.RemoveAll(repoDir); err != nil {
		return core.Project{}, fmt.Errorf("demo: clearing %s: %w", repoDir, err)
	}
	if err := buildRepo(ctx, repoDir); err != nil {
		return core.Project{}, fmt.Errorf("demo: seeding fixture repo: %w", err)
	}

	p := core.Project{ID: ProjectID, Name: projectName, RepoPath: repoDir}
	if err := reg.Remove(ctx, ProjectID); err != nil && !errors.Is(err, core.ErrProjectNotFound) {
		return core.Project{}, fmt.Errorf("demo: clearing previous registration: %w", err)
	}
	if err := reg.Add(ctx, p); err != nil {
		return core.Project{}, fmt.Errorf("demo: registering project: %w", err)
	}
	return p, nil
}

// demoDir is the demo fixture repo's fixed location. Reusing the same path
// on every call is what makes Seed's "wipe and rebuild" idempotency simple.
//
// ponytail: one fixed path assumes a single demo instance per machine — fine
// for FlightDeck's single-user, self-hosted scope (docs/PRD.md §2); make it
// caller-configurable if concurrent demo instances are ever needed.
func demoDir() string {
	return filepath.Join(os.TempDir(), "flightdeck-demo")
}

// ticketSpec is one docs/tickets file to write.
type ticketSpec struct {
	id      int
	slug    string
	title   string
	role    string
	depends []int
	status  string
	body    string
}

// filename is the docs/tickets file a ticketSpec writes to.
func filename(id int, slug string) string { return fmt.Sprintf("%03d-%s.md", id, slug) }

// branchNameFor follows the convention internal/api/board.go's
// ticketFileOnBranch depends on: a claude/NNN-slug branch's ticket file is
// docs/tickets/NNN-slug.md, derived from the branch name alone.
func branchNameFor(id int, slug string) string {
	return "claude/" + strings.TrimSuffix(filename(id, slug), ".md")
}

// withStatus returns a copy of ts with status overridden and extraBody
// appended, for writing a ticket's file differently on a branch than on
// main (simulating an agent's work).
func withStatus(ts ticketSpec, status, extraBody string) ticketSpec {
	ts.status = status
	if extraBody != "" {
		ts.body = ts.body + "\n\n" + extraBody
	}
	return ts
}

// branchSpec is one branch to create from main. tickets rewrites those
// tickets' files on the branch and commits (empty means "claimed by pushing
// the branch, no work committed yet" — in_progress); merged folds the
// branch back into main with --no-ff right after.
type branchSpec struct {
	name    string
	tickets []ticketSpec
	merged  bool
}

// demoTickets is the demo project's ticket queue on main: a small fictional
// invoicing service, todo across the board until demoBranches works some of
// them.
var demoTickets = []ticketSpec{
	{id: 1, slug: "auth-scaffold", title: "Auth scaffold", role: "backend", status: "todo",
		body: "Session auth middleware and the login handler the rest of the API sits behind."},
	{id: 2, slug: "invoice-api", title: "Invoice API", role: "backend", depends: []int{1}, status: "todo",
		body: "CRUD endpoints for invoices, behind the new auth middleware."},
	{id: 3, slug: "pdf-export", title: "PDF export", role: "backend", depends: []int{1}, status: "todo",
		body: "Render an invoice as a downloadable PDF."},
	{id: 4, slug: "rate-limiter", title: "Rate limiter", role: "backend", depends: []int{1}, status: "todo",
		body: "Per-token rate limiting on the public API."},
	{id: 5, slug: "billing-dashboard", title: "Billing dashboard", role: "frontend", depends: []int{1}, status: "todo",
		body: "Customer-facing dashboard summarising invoices and payment status."},
	{id: 6, slug: "webhooks", title: "Webhooks", role: "backend", depends: []int{2}, status: "todo",
		body: "Outbound webhooks for invoice.paid and invoice.overdue, built on the invoice API."},
}

// demoBranches drives every status a branch can produce: ticket 1 done and
// merged to main, 2 in review (branch says done, main doesn't yet), 3 in
// progress (branch claimed, untouched), 4 needs attention. Tickets 5 and 6
// stay branchless on purpose — 5 is ready (its one dependency, 1, is done on
// main), 6 is blocked (its dependency, 2, is not).
var demoBranches = []branchSpec{
	{
		name: branchNameFor(1, "auth-scaffold"),
		tickets: []ticketSpec{withStatus(demoTickets[0], "done",
			"## Handoff\n\nAuth middleware is `RequireSession`; every other route wraps in it.")},
		merged: true,
	},
	{
		name:    branchNameFor(2, "invoice-api"),
		tickets: []ticketSpec{withStatus(demoTickets[1], "done", "")},
	},
	{
		name: branchNameFor(3, "pdf-export"),
		// No commit on purpose: the branch is claimed but nothing has
		// changed yet, so its ticket file still reads "todo" from main.
	},
	{
		name:    branchNameFor(4, "rate-limiter"),
		tickets: []ticketSpec{withStatus(demoTickets[3], "needs-attention", "")},
	},
}

// buildRepo builds the demo fixture repo at repoDir: git init, the initial
// ticket commit on main, then each of demoBranches in order. It mirrors
// internal/source/git/fixture.go's NewFixtureRepo, but as ordinary runtime
// code (no *testing.T) since Seed runs from `flightdeck serve --demo`, not a
// test binary.
func buildRepo(ctx context.Context, repoDir string) error {
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", repoDir, err)
	}
	run := func(args ...string) error { return runGit(ctx, repoDir, args...) }

	for _, args := range [][]string{
		{"init", "-q"},
		{"symbolic-ref", "HEAD", "refs/heads/main"},
		{"config", "user.email", "flightdeck-demo@example.com"},
		{"config", "user.name", "FlightDeck Demo"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := run(args...); err != nil {
			return err
		}
	}

	for _, ts := range demoTickets {
		if err := writeTicket(repoDir, ts); err != nil {
			return fmt.Errorf("writing ticket %d: %w", ts.id, err)
		}
	}
	if err := writeAutopilotFile(repoDir); err != nil {
		return fmt.Errorf("writing autopilot file: %w", err)
	}
	if err := run("add", "-A"); err != nil {
		return err
	}
	if err := run("commit", "-q", "-m", "queue: seed demo tickets"); err != nil {
		return err
	}

	for _, b := range demoBranches {
		if err := run("checkout", "-q", "main"); err != nil {
			return err
		}
		if err := run("checkout", "-q", "-b", b.name); err != nil {
			return err
		}
		if len(b.tickets) > 0 {
			for _, ts := range b.tickets {
				if err := writeTicket(repoDir, ts); err != nil {
					return fmt.Errorf("writing ticket %d on %s: %w", ts.id, b.name, err)
				}
			}
			if err := run("add", "-A"); err != nil {
				return err
			}
			if err := run("commit", "-q", "-m", "fixture: "+b.name); err != nil {
				return err
			}
		}
		if b.merged {
			if err := run("checkout", "-q", "main"); err != nil {
				return err
			}
			if err := run("merge", "-q", "--no-ff", "-m", "merge "+b.name, b.name); err != nil {
				return err
			}
		}
	}
	return run("checkout", "-q", "main")
}

func writeTicket(repoDir string, ts ticketSpec) error {
	depends := make([]string, len(ts.depends))
	for i, d := range ts.depends {
		depends[i] = strconv.Itoa(d)
	}
	content := fmt.Sprintf(
		"---\nid: %d\ntitle: %s\nrole: %s\ndepends: [%s]\nstatus: %s\n---\n%s\n",
		ts.id, ts.title, ts.role, strings.Join(depends, ", "), ts.status, ts.body,
	)
	path := filepath.Join(repoDir, "docs", "tickets", filename(ts.id, ts.slug))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644) //nolint:gosec // demo fixture file, not sensitive
}

// writeAutopilotFile seeds .claude/autopilot.json in the demo repo so the
// project's autopilot state can be read and flipped (GET/PUT
// /api/projects/demo/autopilot, and the Fleet toggle). Without it the
// dispatcher has no file to read and those calls fail.
func writeAutopilotFile(repoDir string) error {
	const content = `{
  "enabled": false,
  "maxInFlight": 1,
  "note": "Demo autopilot switch. Flip it from the Fleet card to see the toggle round-trip .claude/autopilot.json."
}
`
	path := filepath.Join(repoDir, ".claude", "autopilot.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644) //nolint:gosec // demo fixture file, not sensitive
}

// runGit shells to the git binary against repoDir, returning combined
// output on failure so a broken step is diagnosable from the server's log.
func runGit(ctx context.Context, repoDir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed binary, args built from constants
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
