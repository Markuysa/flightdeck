package e2e

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// Deliberately fake, short tokens — never a realistic ghp_<36>/sk-<32>
// literal, which would trip CI's "No secrets" job (.github/workflows/ci.yml)
// even inside a test (internal/api/secrets_test.go hit this same
// constraint first).
const (
	e2eRoutineToken = "routine-tok-test"
	e2eGitHubToken  = "gh-tok-test"
)

// credentialShapedPattern mirrors ci.yml's grep, so this test also catches a
// realistic-looking leaked credential on top of the exact configured token
// literals.
var credentialShapedPattern = regexp.MustCompile(`sk-ant-[A-Za-z0-9]{20}|sk-[A-Za-z0-9]{32}|ghp_[A-Za-z0-9]{36}`)

// TestNoTokenShapedStringReachesClientOverHTTP is PRD §6's fifth acceptance
// criterion: "Secrets (routine tokens, GitHub token) never reach the browser
// and never appear in logs or the committed config." A project's secrets
// are set through the REAL registry.Store (registry.Secrets, exactly the
// path a real routine/GitHub token takes), and every covered flow's
// response body (and headers) is scanned for the exact configured token
// values and for a generic credential-shaped string.
func TestNoTokenShapedStringReachesClientOverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 3, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 4, Title: "In progress ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		[]git.FixtureBranch{
			{
				Name: "claude/002-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 2, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
			{Name: "claude/004-fixture"},
		},
	)

	source := newStubbedGitHubSource()
	store := newTempStore(t)
	dispatcher := newStubDispatcher()
	h := newHarness(t, store, source, newStubDispatcherFactory(dispatcher))
	project := h.registerProject("Secrets Fixture", repo.Path, &githubRef{Owner: "acme", Repo: "widgets"})

	source.setPRs(project.ID, map[string]core.PRState{
		"claude/002-fixture": {Number: 9, URL: "https://github.com/acme/widgets/pull/9", CI: "green"},
	})

	if err := store.SetSecrets(context.Background(), project.ID, registry.Secrets{
		RoutineToken: e2eRoutineToken,
		GitHubToken:  e2eGitHubToken,
	}); err != nil {
		t.Fatalf("SetSecrets: %v", err)
	}

	type call struct {
		method, path string
		body         any
	}
	calls := []call{
		{http.MethodGet, "/api/projects", nil},
		{http.MethodGet, "/api/projects/" + project.ID + "/board", nil},
		{http.MethodGet, "/api/projects/" + project.ID + "/tickets/2", nil},
		{http.MethodGet, "/api/projects/" + project.ID + "/tickets/3", nil},
		{http.MethodGet, "/api/agents", nil},
		{http.MethodPost, "/api/projects/" + project.ID + "/dispatch", api.DispatchRequest{TicketID: 3}},
		{http.MethodPost, "/api/projects/" + project.ID + "/tickets/2/approve", nil},
		// The new write-side routes (ticket 016) over real HTTP, plus a
		// second registration carrying tokens in the request body itself.
		{http.MethodPut, "/api/projects/" + project.ID + "/secrets", api.SetSecretsRequest{
			RoutineToken: e2eRoutineToken, GitHubToken: e2eGitHubToken,
		}},
		{http.MethodGet, "/api/projects/" + project.ID + "/secrets", nil},
		{http.MethodPost, "/api/projects", map[string]any{
			"name": "Secrets Fixture 2", "repo_path": repo.Path,
			"routine_token": e2eRoutineToken, "github_token": e2eGitHubToken,
		}},
	}

	for _, c := range calls {
		resp := h.do(c.method, c.path, c.body)
		body := bodyString(t, resp)
		if strings.Contains(body, e2eRoutineToken) {
			t.Errorf("%s %s response contains the routine token: %s", c.method, c.path, body)
		}
		if strings.Contains(body, e2eGitHubToken) {
			t.Errorf("%s %s response contains the GitHub token: %s", c.method, c.path, body)
		}
		if credentialShapedPattern.MatchString(body) {
			t.Errorf("%s %s response contains a credential-shaped string: %s", c.method, c.path, body)
		}
		for key, values := range resp.Header {
			for _, v := range values {
				if strings.Contains(v, e2eRoutineToken) || strings.Contains(v, e2eGitHubToken) {
					t.Errorf("%s %s response header %s leaked a token: %q", c.method, c.path, key, v)
				}
			}
		}
	}
}
