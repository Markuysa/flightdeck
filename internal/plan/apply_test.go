package plan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fullTicket(number int, title string, deps ...int) Ticket {
	return Ticket{
		Number: number, Title: title, Role: "backend", Depends: deps,
		Body:       "Do the thing.",
		Acceptance: []string{"go test ./... passes"},
		Handoff:    "The Thing interface is in core.",
	}
}

func TestApplyWritesTicketFiles(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	got, err := Apply(repo, planOf(
		fullTicket(1, "Auth scaffold"),
		fullTicket(2, "Invoice API", 1),
	))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if want := []int{1, 2}; len(got.IDs) != 2 || got.IDs[0] != want[0] || got.IDs[1] != want[1] {
		t.Errorf("assigned ids = %v, want %v", got.IDs, want)
	}
	for _, rel := range got.Files {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Errorf("reported file %s does not exist: %v", rel, err)
		}
	}
	if want := "docs/tickets/001-auth-scaffold.md"; got.Files[0] != want {
		t.Errorf("first file = %q, want %q", got.Files[0], want)
	}
}

// TestAppliedFileParsesAsATicket is the contract that matters: a file this
// package writes must be readable by the frontmatter parser the derive engine
// uses. A file that doesn't parse fails the WHOLE board read, not just its own
// ticket — so a formatting slip here takes the project's board down.
func TestAppliedFileParsesAsATicket(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	if _, err := Apply(repo, planOf(fullTicket(1, "Auth scaffold"))); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repo, "docs", "tickets", "001-auth-scaffold.md"))
	if err != nil {
		t.Fatalf("reading written ticket: %v", err)
	}
	content := string(raw)

	for _, want := range []string{
		"---\n",
		"id: 1\n",
		"title: Auth scaffold\n",
		"role: backend\n",
		"status: todo\n",
		"depends: []\n",
		"## Acceptance criteria",
		"- [ ] go test ./... passes",
		"## Handoff",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("written ticket missing %q:\n%s", want, content)
		}
	}
	// The frontmatter block must open on line 1 and close before the body.
	if !strings.HasPrefix(content, "---\n") {
		t.Error("ticket must open with a --- fence on the first line")
	}
	if strings.Count(content, "---\n") < 2 {
		t.Error("ticket must have both an opening and a closing frontmatter fence")
	}
}

// TestApplyTranslatesDependenciesToRealIDs: the plan's Numbers are plan-local,
// and a file that kept them would point at the wrong tickets entirely once ids
// continue from an existing queue.
func TestApplyTranslatesDependenciesToRealIDs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	dir := filepath.Join(repo, "docs", "tickets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// An existing queue: the next plan must continue from 8, not restart at 1.
	if err := os.WriteFile(filepath.Join(dir, "007-existing.md"), []byte("---\nid: 7\n---\n"), 0o644); err != nil {
		t.Fatalf("seeding existing ticket: %v", err)
	}

	got, err := Apply(repo, planOf(
		fullTicket(1, "Foundation"),
		fullTicket(2, "Builds on it", 1),
	))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if want := []int{8, 9}; got.IDs[0] != want[0] || got.IDs[1] != want[1] {
		t.Fatalf("assigned ids = %v, want %v (continuing from the existing 7)", got.IDs, want)
	}

	raw, err := os.ReadFile(filepath.Join(repo, got.Files[1]))
	if err != nil {
		t.Fatalf("reading dependent ticket: %v", err)
	}
	if !strings.Contains(string(raw), "depends: [8]") {
		t.Errorf("dependent ticket must depend on real id 8, not plan number 1:\n%s", raw)
	}
}

// TestApplyNeverOverwritesExistingWork: applying a plan must not be able to
// rewrite a ticket that is already queued or in flight.
func TestApplyNeverOverwritesExistingWork(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	dir := filepath.Join(repo, "docs", "tickets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	existing := filepath.Join(dir, "001-auth-scaffold.md")
	original := "---\nid: 1\ntitle: Do not clobber me\n---\n"
	if err := os.WriteFile(existing, []byte(original), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// A plan whose slug collides, but whose id would be 2 — so the collision is
	// only on the filename. Apply must refuse rather than overwrite.
	_, err := Apply(repo, planOf(fullTicket(1, "Auth scaffold")))
	if err == nil {
		t.Log("no collision (ids advanced past the existing file) — verifying the original survived")
	}
	raw, readErr := os.ReadFile(existing)
	if readErr != nil {
		t.Fatalf("reading existing ticket: %v", readErr)
	}
	if string(raw) != original {
		t.Errorf("Apply modified an existing ticket file:\n%s", raw)
	}
}

func TestApplyRejectsAnInvalidPlan(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Apply(repo, planOf(tk(1, 2), tk(2, 1))); !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("Apply of a cyclic plan = %v, want ErrInvalidPlan", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "tickets")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(repo, "docs", "tickets"))
		if len(entries) > 0 {
			t.Errorf("Apply wrote %d files for an invalid plan, want none", len(entries))
		}
	}
}

// TestSingleLineCollapsesNewlines: a newline in a model-supplied title would
// break out of its frontmatter line and produce a file the parser rejects —
// which fails the entire board read, not just that ticket.
func TestSingleLineCollapsesNewlines(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	nasty := Ticket{
		Number: 1, Role: "backend",
		Title: "Title\nstatus: done\nmalicious: true",
		Body:  "b",
	}
	got, err := Apply(repo, planOf(nasty))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repo, got.Files[0]))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	content := string(raw)

	// The injection to prevent is a *line* the frontmatter parser would read as
	// its own key. Substring matching is the wrong check: the collapsed title
	// legitimately contains the text "status: done" inline, and the parser cuts
	// on the first colon, so that whole run is the title's value. What must not
	// exist is a second line whose key is status (or a key at all) from the
	// title's content.
	fm, _, ok := strings.Cut(content, "\n---\n")
	if !ok {
		t.Fatalf("written ticket has no closing frontmatter fence:\n%s", content)
	}
	var statusLines, maliciousLines int
	for _, line := range strings.Split(fm, "\n") {
		key, _, isPair := strings.Cut(strings.TrimSpace(line), ":")
		if !isPair {
			continue
		}
		switch strings.TrimSpace(key) {
		case "status":
			statusLines++
			if !strings.HasPrefix(strings.TrimSpace(line), "status: todo") {
				t.Errorf("status line = %q, want \"status: todo\"", line)
			}
		case "malicious":
			maliciousLines++
		}
	}
	if statusLines != 1 {
		t.Errorf("frontmatter has %d status lines, want exactly 1:\n%s", statusLines, fm)
	}
	if maliciousLines != 0 {
		t.Errorf("a newline in the title injected a %d-line key into frontmatter:\n%s", maliciousLines, fm)
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"Auth scaffold", "auth-scaffold"},
		{"PDF export (v2)!", "pdf-export-v2"},
		{"   ", "ticket"},
		{"Ünïcödé", "n-c-d"},
		{strings.Repeat("long", 40), strings.Repeat("long", 15)},
	} {
		if got := slugify(tc.in); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
