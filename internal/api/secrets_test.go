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
		// POST /api/projects with tokens (ticket 016): the create response
		// must redact these exactly like every other endpoint.
		{http.MethodPost, "/api/projects", CreateProjectRequest{
			Name: "Newco", RepoPath: "/repos/newco",
			RoutineToken: secretRoutineToken, GitHubToken: secretGitHubToken,
		}},
		{http.MethodGet, "/api/projects/acme/board", nil},
		{http.MethodGet, "/api/projects/acme/tickets/1", nil},
		{http.MethodGet, "/api/projects/acme/tickets/2", nil},
		{http.MethodGet, "/api/agents", nil},
		{http.MethodGet, "/api/projects/acme/autopilot", nil},
		{http.MethodPut, "/api/projects/acme/autopilot", AutopilotState{On: true}},
		{http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}},
		{http.MethodPost, "/api/projects/acme/tickets/2/approve", nil},
		// The two new secrets routes (ticket 016) are the ones most at risk
		// of leaking a token, so they get the same grep as everything else.
		{http.MethodPut, "/api/projects/acme/secrets", SetSecretsRequest{
			RoutineToken: secretRoutineToken, GitHubToken: secretGitHubToken,
		}},
		{http.MethodGet, "/api/projects/acme/secrets", nil},
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

// TestCreateProjectWithTokensStoresSecrets is the ticket's first acceptance
// criterion: POST /api/projects with routine_token/github_token stores them
// via SetSecrets, and the response is the bare core.Project (no token
// field to even check — decodeJSON below would fail to compile one in).
func TestCreateProjectWithTokensStoresSecrets(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()

	rec := doRequest(t, h, http.MethodPost, "/api/projects", CreateProjectRequest{
		Name: "Acme", RepoPath: "/repos/acme",
		RoutineToken: secretRoutineToken, GitHubToken: secretGitHubToken,
	}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/projects = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	created := decodeJSON[core.Project](t, rec)

	sec, err := ts.registry.Secrets(context.Background(), created.ID)
	must(t, err)
	if sec.RoutineToken != secretRoutineToken || sec.GitHubToken != secretGitHubToken {
		t.Fatalf("stored secrets = %+v, want routine=%q github=%q", sec, secretRoutineToken, secretGitHubToken)
	}
}

// TestSetSecretsOverlaysOnlyProvidedFields is the PUT route's documented
// semantics: an omitted/empty field in the request leaves the current token
// unchanged rather than blanking it.
func TestSetSecretsOverlaysOnlyProvidedFields(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	ctx := context.Background()

	must(t, ts.registry.Add(ctx, core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	must(t, ts.registry.SetSecrets(ctx, "acme", registry.Secrets{
		RoutineToken: secretRoutineToken,
		GitHubToken:  secretGitHubToken,
	}))

	const rotatedGitHubToken = "gh-tok-test-2"
	rec := doRequest(t, h, http.MethodPut, "/api/projects/acme/secrets", SetSecretsRequest{
		GitHubToken: rotatedGitHubToken,
	}, ts.token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT .../secrets = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	sec, err := ts.registry.Secrets(ctx, "acme")
	must(t, err)
	if sec.RoutineToken != secretRoutineToken {
		t.Errorf("routine token = %q, want unchanged %q", sec.RoutineToken, secretRoutineToken)
	}
	if sec.GitHubToken != rotatedGitHubToken {
		t.Errorf("github token = %q, want rotated to %q", sec.GitHubToken, rotatedGitHubToken)
	}
}

// TestGetSecretsStatusReportsBooleansOnly is the GET route's contract:
// {routine_token_set, github_token_set}, computed from whether the stored
// value is non-empty, never the value itself.
func TestGetSecretsStatusReportsBooleansOnly(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	ctx := context.Background()

	must(t, ts.registry.Add(ctx, core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/secrets", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../secrets (no tokens set) = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	status := decodeJSON[SecretsStatus](t, rec)
	if status.RoutineTokenSet || status.GitHubTokenSet {
		t.Fatalf("status = %+v, want both false before any token is set", status)
	}

	must(t, ts.registry.SetSecrets(ctx, "acme", registry.Secrets{RoutineToken: secretRoutineToken}))

	rec = doRequest(t, h, http.MethodGet, "/api/projects/acme/secrets", nil, ts.token)
	status = decodeJSON[SecretsStatus](t, rec)
	if !status.RoutineTokenSet || status.GitHubTokenSet {
		t.Fatalf("status = %+v, want routine=true github=false", status)
	}
}

// TestSecretsRoutesReturn404ForUnknownProject covers both new routes'
// not-found behaviour.
func TestSecretsRoutesReturn404ForUnknownProject(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()

	putRec := doRequest(t, h, http.MethodPut, "/api/projects/nope/secrets", SetSecretsRequest{
		RoutineToken: secretRoutineToken,
	}, ts.token)
	if putRec.Code != http.StatusNotFound {
		t.Errorf("PUT /api/projects/nope/secrets = %d, want 404", putRec.Code)
	}

	getRec := doRequest(t, h, http.MethodGet, "/api/projects/nope/secrets", nil, ts.token)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("GET /api/projects/nope/secrets = %d, want 404", getRec.Code)
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
