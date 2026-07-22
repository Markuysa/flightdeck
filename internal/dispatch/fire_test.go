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
			Body:       io.NopCloser(strings.NewReader(`{"session_url":"https://sessions.example.test/abc123"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := New("routine-token", "gh-token",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRoutineBaseURL("https://routines.example.test"))

	sessionURL, err := c.Fire(context.Background(), core.Project{ID: "widgets"}, 42)
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if sessionURL != "https://sessions.example.test/abc123" {
		t.Errorf("Fire() sessionURL = %q, want https://sessions.example.test/abc123", sessionURL)
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

func TestFirePostsToRoutineBaseURLSlashFire(t *testing.T) {
	t.Parallel()
	var gotURL string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"session_url":"x"}`)), Header: make(http.Header)}, nil
	})
	// A trailing slash on the configured base URL must not produce "//fire".
	c := New("tok", "gh", WithHTTPClient(&http.Client{Transport: transport}), WithRoutineBaseURL("https://routines.example.test/"))

	if _, err := c.Fire(context.Background(), core.Project{}, 1); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if want := "https://routines.example.test/fire"; gotURL != want {
		t.Errorf("Fire() requested %q, want %q", gotURL, want)
	}
}

func TestFireNonSuccessStatusIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"POST https://routines.example.test/fire": {status: 500, body: `{"error":"boom"}`},
	}
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineBaseURL("https://routines.example.test"))

	_, err := c.Fire(context.Background(), core.Project{}, 1)
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
		WithRoutineBaseURL("https://routines.example.test"))

	_, err := c.Fire(context.Background(), core.Project{}, 1)
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
		"POST https://routines.example.test/fire": {status: 200, body: "not json"},
	}
	c := New("tok", "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineBaseURL("https://routines.example.test"))

	_, err := c.Fire(context.Background(), core.Project{}, 1)
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
		"POST https://routines.example.test/fire": {status: 401, body: `{"error":"unauthorized"}`},
	}
	c := New(secretToken, "gh",
		WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}),
		WithRoutineBaseURL("https://routines.example.test"))

	_, err := c.Fire(context.Background(), core.Project{}, 1)
	if err == nil {
		t.Fatal("Fire() error = nil, want an error")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Errorf("Fire() error = %q, must never contain the token", err.Error())
	}
}
