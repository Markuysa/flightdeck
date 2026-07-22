package github

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

// roundTripFunc lets a test act as an http.RoundTripper without a real
// network call or an httptest.Server.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// stubResponse is a canned reply for one exact request URL.
type stubResponse struct {
	status int
	body   string
	err    error // when set, the transport fails instead of responding
}

// newStubTransport returns a RoundTripper that answers each request by
// looking up its full URL (including query string) in responses. A request
// to any other URL fails the test immediately — every request OpenPRs makes
// must be accounted for.
func newStubTransport(t *testing.T, responses map[string]stubResponse) roundTripFunc {
	t.Helper()
	return func(r *http.Request) (*http.Response, error) {
		key := r.URL.String()
		resp, ok := responses[key]
		if !ok {
			t.Fatalf("unexpected request to %s", key)
			return nil, nil
		}
		if resp.err != nil {
			return nil, resp.err
		}
		return &http.Response{
			StatusCode: resp.status,
			Body:       io.NopCloser(strings.NewReader(resp.body)),
			Header:     make(http.Header),
		}, nil
	}
}

func TestOpenPRsReturnsStatesKeyedByHeadBranch(t *testing.T) {
	t.Parallel()
	const prsBody = `[
		{"number": 7, "html_url": "https://github.com/acme/widgets/pull/7", "head": {"ref": "claude/007-feature", "sha": "sha-green"}},
		{"number": 9, "html_url": "https://github.com/acme/widgets/pull/9", "head": {"ref": "claude/009-feature", "sha": "sha-red"}}
	]`
	const greenRuns = `{"total_count": 1, "check_runs": [{"status": "completed", "conclusion": "success"}]}`
	const redRuns = `{"total_count": 1, "check_runs": [{"status": "completed", "conclusion": "failure"}]}`

	responses := map[string]stubResponse{
		"https://api.github.com/repos/acme/widgets/pulls?state=open&per_page=100": {status: 200, body: prsBody},
		"https://api.github.com/repos/acme/widgets/commits/sha-green/check-runs":  {status: 200, body: greenRuns},
		"https://api.github.com/repos/acme/widgets/commits/sha-red/check-runs":    {status: 200, body: redRuns},
	}
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))

	got, err := c.OpenPRs(context.Background())
	if err != nil {
		t.Fatalf("OpenPRs: %v", err)
	}

	want := map[string]core.PRState{
		"claude/007-feature": {Number: 7, URL: "https://github.com/acme/widgets/pull/7", CI: "green"},
		"claude/009-feature": {Number: 9, URL: "https://github.com/acme/widgets/pull/9", CI: "red"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OpenPRs() = %+v, want %+v", got, want)
	}
}

func TestOpenPRsNoOpenPRsReturnsEmptyMap(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"https://api.github.com/repos/acme/widgets/pulls?state=open&per_page=100": {status: 200, body: `[]`},
	}
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))

	got, err := c.OpenPRs(context.Background())
	if err != nil {
		t.Fatalf("OpenPRs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("OpenPRs() = %+v, want an empty map", got)
	}
}

func TestOpenPRsSendsExpectedRequestHeaders(t *testing.T) {
	t.Parallel()
	const token = "test-token"
	var gotAuth, gotAccept, gotVersion string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`)), Header: make(http.Header)}, nil
	})
	c := New("acme", "widgets", token, WithHTTPClient(&http.Client{Transport: transport}))

	if _, err := c.OpenPRs(context.Background()); err != nil {
		t.Fatalf("OpenPRs: %v", err)
	}

	if want := "Bearer " + token; gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept header = %q, want application/vnd.github+json", gotAccept)
	}
	if gotVersion != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version header = %q, want 2022-11-28", gotVersion)
	}
}

func TestOpenPRsAPIErrorIsTypedAndDowngradable(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"https://api.github.com/repos/acme/widgets/pulls?state=open&per_page=100": {status: 500, body: `{"message": "internal error"}`},
	}
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))

	_, err := c.OpenPRs(context.Background())
	if err == nil {
		t.Fatal("OpenPRs() error = nil, want ErrGitHubUnavailable")
	}
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("OpenPRs() error = %v, want it to wrap ErrGitHubUnavailable", err)
	}
}

func TestOpenPRsTransportErrorIsTypedAndDowngradable(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("connection refused")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, wantErr
	})
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: transport}))

	_, err := c.OpenPRs(context.Background())
	if err == nil {
		t.Fatal("OpenPRs() error = nil, want ErrGitHubUnavailable")
	}
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("OpenPRs() error = %v, want it to wrap ErrGitHubUnavailable", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("OpenPRs() error = %v, want it to wrap the transport error %v", err, wantErr)
	}
}

func TestOpenPRsMalformedBodyIsTypedError(t *testing.T) {
	t.Parallel()
	responses := map[string]stubResponse{
		"https://api.github.com/repos/acme/widgets/pulls?state=open&per_page=100": {status: 200, body: "not json"},
	}
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))

	_, err := c.OpenPRs(context.Background())
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("OpenPRs() error = %v, want it to wrap ErrGitHubUnavailable", err)
	}
}

// TestTokenNeverAppearsInErrorMessages guards the secrets rule in
// CLAUDE.md: a token must never be logged, and OpenPRs' errors are the one
// place a caller might be tempted to log verbatim.
func TestTokenNeverAppearsInErrorMessages(t *testing.T) {
	t.Parallel()
	const secretToken = "ghp_supersecrettoken12345"
	responses := map[string]stubResponse{
		"https://api.github.com/repos/acme/widgets/pulls?state=open&per_page=100": {status: 401, body: `{"message": "Bad credentials"}`},
	}
	c := New("acme", "widgets", secretToken, WithHTTPClient(&http.Client{Transport: newStubTransport(t, responses)}))

	_, err := c.OpenPRs(context.Background())
	if err == nil {
		t.Fatal("OpenPRs() error = nil, want an error")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Errorf("OpenPRs() error = %q, must never contain the token", err.Error())
	}
}
