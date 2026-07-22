package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// Test tokens are deliberately fake and short — CI's "No secrets" job
// (.github/workflows/ci.yml) greps every tracked file for credential-shaped
// strings (ghp_<36>, sk-ant-<20>, sk-<32>), and a realistic token literal
// anywhere, even in a test, would trip it (registry_test.go hit this same
// constraint first; see its testRoutineToken/testGitHubToken).
const (
	secretRoutineToken = "routine-tok-test"
	secretGitHubToken  = "gh-tok-test"
)

// credentialShapedPattern mirrors ci.yml's grep so this test also catches a
// realistic-looking leaked credential, on top of the exact
// configured token literals.
var credentialShapedPattern = regexp.MustCompile(`sk-ant-[A-Za-z0-9]{20}|sk-[A-Za-z0-9]{32}|ghp_[A-Za-z0-9]{36}`)

// TestSecretsNeverAppearInAnyHandlerResponse is the ticket's acceptance
// criterion: every documented endpoint's response body is grepped for the
// configured secret token values and for generic credential-shaped strings.
// None of api's DTOs carry a token field by construction (see types.ts and
// internal/registry's own redaction), so this is a regression guard, not a
// currently-failing probe.
func TestSecretsNeverAppearInAnyHandlerResponse(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	ctx := context.Background()

	must(t, ts.registry.Add(ctx, core.Project{
		ID: "acme", Name: "Acme", RepoPath: "/repos/acme",
		Remote: "github", Owner: "acme", Repo: "widgets",
	}))
	must(t, ts.registry.SetSecrets(ctx, "acme", registry.Secrets{
		RoutineToken: secretRoutineToken,
		GitHubToken:  secretGitHubToken,
	}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Ready"}, Status: core.StatusReady},
		{
			Ticket: core.Ticket{ID: 2, Title: "In review"}, Status: core.StatusInReview,
			Branch: "claude/002-in-review",
			PR:     &core.PRState{Number: 9, URL: "https://github.com/acme/widgets/pull/9", CI: "green"},
		},
	})
	fake := ts.dispatcher.forProject("acme")
	fake.sessionURL = "https://routines.example.com/sessions/abc123"

	type call struct {
		method, path string
		body         any
	}
	calls := []call{
		{http.MethodGet, "/api/projects", nil},
		{http.MethodPost, "/api/projects", CreateProjectRequest{Name: "Other", RepoPath: "/repos/other"}},
		{http.MethodGet, "/api/projects/acme/board", nil},
		{http.MethodGet, "/api/projects/acme/tickets/1", nil},
		{http.MethodGet, "/api/projects/acme/tickets/2", nil},
		{http.MethodGet, "/api/agents", nil},
		{http.MethodGet, "/api/projects/acme/autopilot", nil},
		{http.MethodPut, "/api/projects/acme/autopilot", AutopilotState{On: true}},
		{http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}},
		{http.MethodPost, "/api/projects/acme/tickets/2/approve", nil},
	}

	for _, c := range calls {
		rec := doRequest(t, h, c.method, c.path, c.body, ts.token)
		body := rec.Body.String()
		if strings.Contains(body, secretRoutineToken) {
			t.Errorf("%s %s response contains the routine token: %s", c.method, c.path, body)
		}
		if strings.Contains(body, secretGitHubToken) {
			t.Errorf("%s %s response contains the GitHub token: %s", c.method, c.path, body)
		}
		if credentialShapedPattern.MatchString(body) {
			t.Errorf("%s %s response contains a credential-shaped string: %s", c.method, c.path, body)
		}
		// Also check response headers (e.g. an accidental echo into a
		// custom header) for good measure.
		for key, values := range rec.Header() {
			for _, v := range values {
				if strings.Contains(v, secretRoutineToken) || strings.Contains(v, secretGitHubToken) {
					t.Errorf("%s %s response header %s leaked a token: %q", c.method, c.path, key, v)
				}
			}
		}
	}
}

func TestEventsRouteRequiresAuth(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	ts.srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/events with no auth = %d, want 401", rec.Code)
	}
}
