// Package e2e proves PRD §6's top-level acceptance criteria end to end
// against a fixture project (ticket 014): it boots the REAL api.Server
// behind a REAL HTTP listener (httptest), drives it over REAL HTTP with the
// same cookie-auth flow the UI uses, and keeps status derivation (git +
// derive.Derive) REAL throughout — only the routine dispatch endpoint and
// GitHub's PR/CI state are stubbed at the boundary, per the ticket's
// instructions. Every test builds its own temp git fixture (git.NewFixtureRepo)
// and its own temp-file registry (registry.Open), so the suite has no shared
// state, no network access, and no sleeps — deterministic under
// `go test -race -count=1 ./...`.
package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// e2eToken stands in for FLIGHTDECK_TOKEN across this suite: short and
// obviously fake so it can never be mistaken for, or need to be scanned as,
// a real credential.
const e2eToken = "e2e-test-token"

// fdSessionCookie mirrors internal/api/auth.go's unexported
// sessionCookieName constant — the cookie POST /api/session issues and
// every other route accepts in place of the bearer token.
const fdSessionCookie = "fd_session"

// harness boots a real api.Server behind a real HTTP listener and
// authenticates the way the UI does (bearer -> session cookie), giving
// tests a small client to drive it over real HTTP — ticket 014's central
// requirement.
type harness struct {
	t      *testing.T
	ts     *httptest.Server
	client *http.Client
	cookie *http.Cookie
}

// newTempStore opens a fresh temp-file registry (t.TempDir(), never
// ":memory:", per registry.Open's own doc comment) that the caller can also
// use directly (e.g. store.SetSecrets) before or after wiring it into a
// harness.
func newTempStore(t *testing.T) *registry.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	store, err := registry.Open(dbPath)
	if err != nil {
		t.Fatalf("opening registry: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// newHarness builds api.NewServer from the given store/source/dispatcher —
// exactly the injectable seams ticket 014 names — and boots it behind a
// real httptest.Server, then trades e2eToken for a session cookie.
func newHarness(t *testing.T, store *registry.Store, source api.ProjectSource, dispatcher api.DispatcherFactory) *harness {
	t.Helper()

	srv := api.NewServer(api.Config{
		Token:      e2eToken,
		Registry:   store,
		Source:     source,
		Dispatcher: dispatcher,
	})

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	h := &harness{t: t, ts: ts, client: ts.Client()}
	h.cookie = h.createSession()
	return h
}

// createSession trades the bearer token for a session cookie exactly as the
// UI does (docs/ARCHITECTURE.md's auth flow), so every later request this
// suite makes exercises cookie auth, not the bearer shortcut.
func (h *harness) createSession() *http.Cookie {
	t := h.t
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("building session request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e2eToken)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /api/session = %d, want 204", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == fdSessionCookie {
			return c
		}
	}
	t.Fatal("POST /api/session issued no session cookie")
	return nil
}

// do sends method/path with an optional JSON body, authenticated by the
// session cookie createSession obtained, and returns the raw response. The
// caller is responsible for reading/closing the body (decodeJSON and
// bodyString both do so).
func (h *harness) do(method, path string, body any) *http.Response {
	t := h.t
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.ts.URL+path, reader)
	if err != nil {
		t.Fatalf("building %s %s request: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(h.cookie)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// githubRef names a project's GitHub owner/repo for registerProject; nil
// registers a local-only project (core.Project.Remote stays "").
type githubRef struct{ Owner, Repo string }

// registerProject registers a project via POST /api/projects (US-7) and
// returns the created core.Project, exactly the flow the UI follows (and
// ticket 013's handoff documents verbatim).
func (h *harness) registerProject(name, repoPath string, gh *githubRef) core.Project {
	t := h.t
	t.Helper()

	body := map[string]any{"name": name, "repo_path": repoPath}
	if gh != nil {
		body["github"] = map[string]string{"owner": gh.Owner, "repo": gh.Repo}
	}
	resp := h.do(http.MethodPost, "/api/projects", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/projects = %d, want 200: %s", resp.StatusCode, bodyString(t, resp))
	}
	return decodeJSON[core.Project](t, resp)
}

// decodeJSON reads resp's body as JSON into a T, closing the body, and
// fails the test on any decode error.
func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
	return v
}

// bodyString reads resp's body as a string, closing it — used both for
// error messages and for the no-token-leaked assertions, which need the raw
// text to grep rather than a typed decode.
func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return string(b)
}
