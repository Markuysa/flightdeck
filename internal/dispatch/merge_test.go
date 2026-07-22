package dispatch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

func TestApproveMergeSquashMergesThePR(t *testing.T) {
	t.Parallel()
	var gotMethod, gotURL, gotAuth, gotAccept, gotBody string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"merged":true,"message":"squashed"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := New("routine-token", "gh-token", WithHTTPClient(&http.Client{Transport: transport}))
	p := core.Project{ID: "widgets", Owner: "acme", Repo: "widgets"}

	if err := c.ApproveMerge(context.Background(), p, 42); err != nil {
		t.Fatalf("ApproveMerge: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("ApproveMerge() method = %q, want PUT", gotMethod)
	}
	if want := "https://api.github.com/repos/acme/widgets/pulls/42/merge"; gotURL != want {
		t.Errorf("ApproveMerge() URL = %q, want %q", gotURL, want)
	}
	if want := "Bearer gh-token"; gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
	if want := "application/vnd.github+json"; gotAccept != want {
		t.Errorf("Accept header = %q, want %q", gotAccept, want)
	}
	if want := `{"merge_method":"squash"}`; gotBody != want {
		t.Errorf("request body = %q, want %q", gotBody, want)
	}
}

func TestApproveMergeNonSuccessStatusIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"PUT https://api.github.com/repos/acme/widgets/pulls/42/merge": {status: 405, body: `{"message":"Pull Request is not mergeable"}`},
	}
	c := New("rt", "gh", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))
	p := core.Project{Owner: "acme", Repo: "widgets"}

	err := c.ApproveMerge(context.Background(), p, 42)
	if err == nil {
		t.Fatal("ApproveMerge() error = nil, want ErrMergeFailed")
	}
	if !errors.Is(err, ErrMergeFailed) {
		t.Errorf("ApproveMerge() error = %v, want it to wrap ErrMergeFailed", err)
	}
}

func TestApproveMergeGitHubReportsNotMergedIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"PUT https://api.github.com/repos/acme/widgets/pulls/42/merge": {status: 200, body: `{"merged":false,"message":"Merge conflict"}`},
	}
	c := New("rt", "gh", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))
	p := core.Project{Owner: "acme", Repo: "widgets"}

	err := c.ApproveMerge(context.Background(), p, 42)
	if !errors.Is(err, ErrMergeFailed) {
		t.Errorf("ApproveMerge() error = %v, want it to wrap ErrMergeFailed", err)
	}
}

func TestApproveMergeTransportErrorIsTypedError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("connection refused")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, wantErr })
	c := New("rt", "gh", WithHTTPClient(&http.Client{Transport: transport}))
	p := core.Project{Owner: "acme", Repo: "widgets"}

	err := c.ApproveMerge(context.Background(), p, 42)
	if !errors.Is(err, ErrMergeFailed) {
		t.Errorf("ApproveMerge() error = %v, want it to wrap ErrMergeFailed", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("ApproveMerge() error = %v, want it to wrap the transport error %v", err, wantErr)
	}
}

// TestApproveMergeTokenNeverAppearsInErrorMessages guards the secrets rule
// in CLAUDE.md: a token must never be logged, and ApproveMerge's errors are
// the one place a caller might be tempted to log verbatim.
func TestApproveMergeTokenNeverAppearsInErrorMessages(t *testing.T) {
	t.Parallel()
	const secretToken = "ghp_supersecrettoken12345"
	responses := map[string]stubResponse{
		"PUT https://api.github.com/repos/acme/widgets/pulls/42/merge": {status: 401, body: `{"message":"Bad credentials"}`},
	}
	c := New("rt", secretToken, WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))
	p := core.Project{Owner: "acme", Repo: "widgets"}

	err := c.ApproveMerge(context.Background(), p, 42)
	if err == nil {
		t.Fatal("ApproveMerge() error = nil, want an error")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Errorf("ApproveMerge() error = %q, must never contain the token", err.Error())
	}
}

// TestFireAndApproveMergeAreSeparateCodePaths is the critical safety test:
// it proves Fire and ApproveMerge never trigger each other. Dispatching a
// ticket must never merge anything, and approving a merge must never fire a
// routine — CLAUDE.md's "dispatches and merges only on explicit human
// action; no auto-anything on the server."
func TestFireAndApproveMergeAreSeparateCodePaths(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"POST https://routines.example.test/fire":                      {status: 200, body: `{"session_url":"https://sessions.example.test/abc"}`},
		"PUT https://api.github.com/repos/acme/widgets/pulls/42/merge": {status: 200, body: `{"merged":true,"message":"ok"}`},
	}
	rt := &recordingTransport{t: t, responses: responses}
	c := New("routine-token", "gh-token",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithRoutineBaseURL("https://routines.example.test"))
	p := core.Project{ID: "widgets", Owner: "acme", Repo: "widgets"}

	// Firing a ticket must issue only the /fire request — never a merge
	// request.
	if _, err := c.Fire(context.Background(), p, 7); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	for _, req := range rt.seen {
		if strings.Contains(req, "/merge") {
			t.Fatalf("Fire triggered a merge request: %s", req)
		}
	}
	if want := []string{"POST https://routines.example.test/fire"}; !reflect.DeepEqual(rt.seen, want) {
		t.Fatalf("Fire issued requests %v, want exactly %v", rt.seen, want)
	}

	rt.seen = nil // reset the log before exercising ApproveMerge in isolation

	// Approving the merge must issue only the merge request — never a
	// /fire request.
	if err := c.ApproveMerge(context.Background(), p, 42); err != nil {
		t.Fatalf("ApproveMerge: %v", err)
	}
	for _, req := range rt.seen {
		if strings.Contains(req, "/fire") {
			t.Fatalf("ApproveMerge triggered a dispatch request: %s", req)
		}
	}
	if want := []string{"PUT https://api.github.com/repos/acme/widgets/pulls/42/merge"}; !reflect.DeepEqual(rt.seen, want) {
		t.Fatalf("ApproveMerge issued requests %v, want exactly %v", rt.seen, want)
	}
}
