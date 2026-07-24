package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestSmoke_BoardOverHTTP is ticket 013's acceptance criterion: it boots a
// real App (registry + api.Server + embedded UI, wired exactly as New
// wires them for `flightdeck serve`) against a fixture project and gets a
// derived board back over HTTP. It is fully offline: the fixture project
// has no GitHub remote, so gitHubSource's PR reader is never reached (see
// internal/api/board.go's openPRs), and the registry is a temp-file SQLite
// database, not a live one.
func TestSmoke_BoardOverHTTP(t *testing.T) {
	// A small, known ticket graph. git.NewFixtureRepo always names a
	// ticket's file "NNN-fixture.md" regardless of Title (fixture.go's
	// fixtureTicketFilename), so a branch follows the real
	// docs/tickets/README.md convention of sharing that NNN-slug —
	// "claude/004-fixture" reads "docs/tickets/004-fixture.md" (see
	// internal/api/board_test.go's TestGitHubSourceBoardTickets_RealGitFixture,
	// which this mirrors).
	//   1: done on main.
	//   2: depends on 1 (done) -> ready, no branch.
	//   3: depends on 2 (not done) -> blocked, no branch.
	//   4: a claude/004-fixture branch exists, still todo on it -> in_progress.
	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 3, Title: "Blocked ticket", Role: "backend", Depends: []int{2}, Status: "todo"},
			{ID: 4, Title: "In progress ticket", Role: "backend", Status: "todo"},
		},
		[]git.FixtureBranch{
			{Name: "claude/004-fixture"},
		},
	)

	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	const token = "smoke-test-token"
	a, err := New(Config{Token: token, Addr: ":0", DBPath: dbPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Close() }()

	// A real listener on a random port, per the ticket's acceptance
	// criterion ("boots the server ... over HTTP").
	ts := httptest.NewServer(a.Handler())
	defer ts.Close()
	client := ts.Client()

	// Authenticate: trade the bearer token for a session cookie, exactly
	// the flow the UI uses (docs/ARCHITECTURE.md's auth flow, ticket 008's
	// handoff).
	sessionReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("building session request: %v", err)
	}
	sessionReq.Header.Set("Authorization", "Bearer "+token)
	sessionResp, err := client.Do(sessionReq)
	if err != nil {
		t.Fatalf("POST /api/session: %v", err)
	}
	_ = sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /api/session = %d, want 204", sessionResp.StatusCode)
	}
	var sessionCookie *http.Cookie
	for _, c := range sessionResp.Cookies() {
		if c.Name == "fd_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("POST /api/session issued no session cookie")
	}

	// Register the fixture repo as a project (US-7), local-only (no
	// github field, so p.Remote stays "" and PR reading never happens).
	createBody, err := json.Marshal(map[string]string{
		"name":      "Smoke Fixture",
		"repo_path": repo.Path,
	})
	if err != nil {
		t.Fatalf("marshaling create-project body: %v", err)
	}
	createReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/projects", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("building create-project request: %v", err)
	}
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(sessionCookie)
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("POST /api/projects: %v", err)
	}
	defer func() { _ = createResp.Body.Close() }()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/projects = %d, want 200", createResp.StatusCode)
	}
	var project core.Project
	if err := json.NewDecoder(createResp.Body).Decode(&project); err != nil {
		t.Fatalf("decoding created project: %v", err)
	}
	if project.ID == "" {
		t.Fatal("created project has an empty ID")
	}

	// GET the derived board over HTTP and check it matches what the
	// fixture implies (derive.Derive's rule table, docs/ARCHITECTURE.md).
	boardReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/projects/"+project.ID+"/board", nil)
	if err != nil {
		t.Fatalf("building board request: %v", err)
	}
	boardReq.AddCookie(sessionCookie)
	boardResp, err := client.Do(boardReq)
	if err != nil {
		t.Fatalf("GET .../board: %v", err)
	}
	defer func() { _ = boardResp.Body.Close() }()
	if boardResp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../board = %d, want 200", boardResp.StatusCode)
	}
	var board map[core.DerivedStatus][]core.BoardTicket
	if err := json.NewDecoder(boardResp.Body).Decode(&board); err != nil {
		t.Fatalf("decoding board: %v", err)
	}

	wantStatus := map[int]core.DerivedStatus{
		1: core.StatusDone,
		2: core.StatusReady,
		3: core.StatusBlocked,
		4: core.StatusInProgress,
	}
	gotStatus := map[int]core.DerivedStatus{}
	for status, tickets := range board {
		for _, bt := range tickets {
			gotStatus[bt.ID] = status
			if bt.Status != status {
				t.Errorf("ticket %d: bucket %q but BoardTicket.Status = %q", bt.ID, status, bt.Status)
			}
		}
	}
	for id, want := range wantStatus {
		if got := gotStatus[id]; got != want {
			t.Errorf("ticket %d derived status = %q, want %q", id, got, want)
		}
	}
}
