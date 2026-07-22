package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

// doRequest sends method/path with an optional JSON body and auth header
// (an empty auth sends no Authorization header) through h, returning the
// recorded response.
func doRequest(t *testing.T, h http.Handler, method, path string, body any, auth string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding response body %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestCreateSessionRejectsBadToken(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/session", nil, "wrong-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/session with bad token = %d, want 401", rec.Code)
	}
}

func TestCreateSessionRejectsNoToken(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/session", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/session with no token = %d, want 401", rec.Code)
	}
}

func TestCreateSessionSetsHTTPOnlyCookie(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/session", nil, ts.token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/session with valid token = %d, want 204", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatalf("no %q cookie in Set-Cookie: %v", sessionCookieName, cookies)
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	if sessionCookie.Value == "" {
		t.Error("session cookie has an empty value")
	}
}

func TestSessionCookieAuthenticatesSubsequentRequests(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()

	sessionRec := doRequest(t, h, http.MethodPost, "/api/session", nil, ts.token)
	var cookie *http.Cookie
	for _, c := range sessionRec.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie issued")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/projects with session cookie = %d, want 200", rec.Code)
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/projects", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/projects with no auth = %d, want 401", rec.Code)
	}
}

func TestProtectedRoutesAcceptBearerToken(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/projects", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/projects with bearer token = %d, want 200", rec.Code)
	}
}

func TestProtectedRoutesRejectWrongBearerToken(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/projects", nil, "not-the-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/projects with wrong bearer token = %d, want 401", rec.Code)
	}
}

func TestCreateAndListAndDeleteProject(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()

	createRec := doRequest(t, h, http.MethodPost, "/api/projects", CreateProjectRequest{
		Name:     "Acme Web",
		RepoPath: "/repos/acme-web",
	}, ts.token)
	if createRec.Code != http.StatusOK {
		t.Fatalf("POST /api/projects = %d, want 200: %s", createRec.Code, createRec.Body.String())
	}
	created := decodeJSON[core.Project](t, createRec)
	if created.ID == "" {
		t.Fatal("created project has an empty ID")
	}
	if created.Name != "Acme Web" || created.RepoPath != "/repos/acme-web" {
		t.Errorf("created project = %+v, want Name/RepoPath preserved", created)
	}

	listRec := doRequest(t, h, http.MethodGet, "/api/projects", nil, ts.token)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/projects = %d, want 200", listRec.Code)
	}
	summaries := decodeJSON[[]ProjectSummary](t, listRec)
	if len(summaries) != 1 || summaries[0].ID != created.ID {
		t.Fatalf("GET /api/projects = %+v, want one summary for %q", summaries, created.ID)
	}
	for _, status := range []core.DerivedStatus{
		core.StatusReady, core.StatusInProgress, core.StatusInReview,
		core.StatusBlocked, core.StatusNeedsAttention, core.StatusDone,
	} {
		if _, ok := summaries[0].Counts[status]; !ok {
			t.Errorf("counts missing key %q", status)
		}
	}

	deleteRec := doRequest(t, h, http.MethodDelete, "/api/projects/"+created.ID, nil, ts.token)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/projects/%s = %d, want 204", created.ID, deleteRec.Code)
	}

	listAfterRec := doRequest(t, h, http.MethodGet, "/api/projects", nil, ts.token)
	afterSummaries := decodeJSON[[]ProjectSummary](t, listAfterRec)
	if len(afterSummaries) != 0 {
		t.Fatalf("GET /api/projects after delete = %+v, want empty", afterSummaries)
	}
}

func TestDeleteUnknownProjectReturns404(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodDelete, "/api/projects/nope", nil, ts.token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE /api/projects/nope = %d, want 404", rec.Code)
	}
}

func TestGetUnknownProjectBoardReturns404(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/projects/nope/board", nil, ts.token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/projects/nope/board = %d, want 404", rec.Code)
	}
}
