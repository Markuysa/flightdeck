package dispatch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

// triggerProject is a project wired to a routine — the only shape Fire
// accepts. Fire on a project without RoutineTriggerID is its own test
// (TestFireWithoutTriggerIsTypedError).
func triggerProject() core.Project {
	return core.Project{ID: "widgets", RoutineTriggerID: "trg_123"}
}

const testRunURL = "https://api.example.test/v1/code/triggers/trg_123/run"

func TestFireReturnsSessionURL(t *testing.T) {
	t.Parallel()
	var gotAuth, gotContentType, gotBody string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"session_url":"https://claude.ai/code/abc123"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := New("routine-token", "gh-token",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRoutineAPIBase("https://api.example.test"))

	sessionURL, err := c.Fire(context.Background(), triggerProject(), 42, core.Briefing{})
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if sessionURL != "https://claude.ai/code/abc123" {
		t.Errorf("Fire() sessionURL = %q, want https://claude.ai/code/abc123", sessionURL)
	}
	if want := "Bearer routine-token"; gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
	if want := "application/json"; gotContentType != want {
		t.Errorf("Content-Type header = %q, want %q", gotContentType, want)
	}
	if want := `{"ticket_id":42}`; gotBody != want {
		t.Errorf("request body = %q, want %q", gotBody, want)
	}
}

// TestFirePostsToTriggerRunEndpoint pins the one URL shape the remote-trigger
// API defines: POST <base>/v1/code/triggers/{trigger_id}/run. A trailing
// slash on the configured base must not produce a doubled separator.
func TestFirePostsToTriggerRunEndpoint(t *testing.T) {
	t.Parallel()
	var gotURL, gotMethod string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL, gotMethod = r.URL.String(), r.Method
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"session_url":"x"}`)), Header: make(http.Header)}, nil
	})
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRoutineAPIBase("https://api.example.test/"))

	if _, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if gotURL != testRunURL {
		t.Errorf("Fire() requested %q, want %q", gotURL, testRunURL)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("Fire() method = %q, want POST", gotMethod)
	}
}

// TestFireEscapesTriggerID: the trigger id comes from an operator-filled
// form field via the registry, so it must never be able to climb out of its
// path segment.
func TestFireEscapesTriggerID(t *testing.T) {
	t.Parallel()
	var gotPath string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.EscapedPath()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRoutineAPIBase("https://api.example.test"))

	p := core.Project{ID: "widgets", RoutineTriggerID: "../../admin/wipe"}
	if _, err := c.Fire(context.Background(), p, 1, core.Briefing{}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if strings.Contains(gotPath, "/admin/wipe") {
		t.Errorf("Fire() path = %q, trigger id escaped its path segment", gotPath)
	}
	if want := "/v1/code/triggers/..%2F..%2Fadmin%2Fwipe/run"; gotPath != want {
		t.Errorf("Fire() path = %q, want %q", gotPath, want)
	}
}

// TestFireWithoutTriggerIsTypedError: a project that was registered without
// a routine trigger must fail with its own sentinel, so the API layer can
// tell "not wired up yet" (409) apart from "the routine rejected us" (502).
func TestFireWithoutTriggerIsTypedError(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("Fire made a request for a project with no trigger: %s", r.URL)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	c := New("tok", "gh", WithHTTPClient(&http.Client{Transport: transport}))

	_, err := c.Fire(context.Background(), core.Project{ID: "widgets"}, 1, core.Briefing{})
	if !errors.Is(err, ErrNoRoutineTrigger) {
		t.Errorf("Fire() error = %v, want it to wrap ErrNoRoutineTrigger", err)
	}
	if errors.Is(err, ErrDispatchFailed) {
		t.Error("Fire() error must not also wrap ErrDispatchFailed: the routine was never called")
	}
}

// TestFireAcceptsAlternateSessionURLSpellings: the remote-trigger API's
// exact field name for the live session link is not pinned down by this
// codebase (ADR-006), so Fire accepts every plausible spelling — and a
// response carrying none is a successful dispatch with no link, not an error.
func TestFireAcceptsAlternateSessionURLSpellings(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, want string }{
		{"session_url", `{"session_url":"https://a.test/1"}`, "https://a.test/1"},
		{"url", `{"url":"https://a.test/2"}`, "https://a.test/2"},
		{"routine_url", `{"routine_url":"https://a.test/3"}`, "https://a.test/3"},
		{"web_url", `{"web_url":"https://a.test/4"}`, "https://a.test/4"},
		{"nested session.url", `{"session":{"url":"https://a.test/5"}}`, "https://a.test/5"},
		{"none of them", `{"id":"run_1","status":"queued"}`, ""},
		{"session_url wins over url", `{"url":"https://a.test/b","session_url":"https://a.test/a"}`, "https://a.test/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			c := New("tok", "gh",
				WithHTTPClient(&http.Client{Transport: transport}),
				WithRoutineAPIBase("https://api.example.test"))

			got, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{})
			if err != nil {
				t.Fatalf("Fire: %v", err)
			}
			if got != tc.want {
				t.Errorf("Fire() sessionURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFireNonSuccessStatusIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"POST " + testRunURL: {status: 500, body: `{"error":"boom"}`},
	}
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineAPIBase("https://api.example.test"))

	_, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{})
	if err == nil {
		t.Fatal("Fire() error = nil, want ErrDispatchFailed")
	}
	if !errors.Is(err, ErrDispatchFailed) {
		t.Errorf("Fire() error = %v, want it to wrap ErrDispatchFailed", err)
	}
}

func TestFireTransportErrorIsTypedError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("connection refused")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, wantErr })
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRoutineAPIBase("https://api.example.test"))

	_, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{})
	if !errors.Is(err, ErrDispatchFailed) {
		t.Errorf("Fire() error = %v, want it to wrap ErrDispatchFailed", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Fire() error = %v, want it to wrap the transport error %v", err, wantErr)
	}
}

func TestFireMalformedResponseIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"POST " + testRunURL: {status: 200, body: "not json"},
	}
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineAPIBase("https://api.example.test"))

	_, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{})
	if !errors.Is(err, ErrDispatchFailed) {
		t.Errorf("Fire() error = %v, want it to wrap ErrDispatchFailed", err)
	}
}

// TestFireTokenNeverAppearsInErrorMessages guards the secrets rule in
// CLAUDE.md: a token must never be logged, and Fire's errors are the one
// place a caller might be tempted to log verbatim.
func TestFireTokenNeverAppearsInErrorMessages(t *testing.T) {
	t.Parallel()
	const secretToken = "routine-secret-abc123"
	responses := map[string]stubResponse{
		"POST " + testRunURL: {status: 401, body: `{"error":"unauthorized"}`},
	}
	c := New(secretToken, "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineAPIBase("https://api.example.test"))

	_, err := c.Fire(context.Background(), triggerProject(), 1, core.Briefing{})
	if err == nil {
		t.Fatal("Fire() error = nil, want an error")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Errorf("Fire() error = %q, must never contain the token", err.Error())
	}
}
