// Package dispatch implements core.Dispatcher: firing a project's routine,
// reading and flipping its autopilot switch, and approving a PR merge — all
// only on explicit human action (CLAUDE.md: "The dashboard dispatches and
// merges only on explicit human action. No auto-anything on the server").
//
// A Client is constructed once per project with that project's routine and
// GitHub tokens (sourced from the registry, ticket 006); the owner/repo and
// local repo path a given call needs come from the core.Project argument
// each method already takes, so one Client is safe to reuse across calls for
// the same project.
//
// Fire (fire.go) and ApproveMerge (merge.go) are two independent methods
// that never call each other or share a code path: Fire only ever issues a
// POST to the routine's /fire endpoint, ApproveMerge only ever issues a PUT
// to GitHub's merge endpoint. See merge.go's doc comment for the invariant
// this encodes and dispatch_test.go's separation test for the proof.
package dispatch

import (
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
)

// defaultRoutineBaseURL is used when the caller does not override it with
// WithRoutineBaseURL. Every deployment of Claude Code Routines is
// self-hosted per team (PRD US-7: registering a project optionally names
// its routine dispatch endpoint), so this is a placeholder, non-routable
// value — the composition root (internal/app) is expected to always
// override it per project.
const defaultRoutineBaseURL = "https://routines.claude.local"

// Client implements core.Dispatcher for one project's routine and GitHub
// tokens.
type Client struct {
	routineToken   string
	githubToken    string
	routineBaseURL string
	httpClient     *http.Client
}

var _ core.Dispatcher = (*Client)(nil)

// Option configures a Client constructed by New.
type Option func(*Client)

// WithHTTPClient overrides the http.Client used for every request. Tests
// pass one whose Transport is a stub http.RoundTripper returning canned
// responses, so no test makes a live network call.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithRoutineBaseURL overrides the routine dispatch endpoint's base URL
// (e.g. "https://routines.example.com"); Fire POSTs "<base>/fire". Callers
// normally set this per project, from wherever US-7's optional routine
// dispatch endpoint is configured.
func WithRoutineBaseURL(url string) Option {
	return func(c *Client) { c.routineBaseURL = url }
}

// New returns a Client that fires routines and merges PRs using
// routineToken and githubToken. Both are constructor parameters only —
// never hardcoded here, never logged (see fire.go's and merge.go's token
// tests) — and the caller (the registry's per-project Secrets, ticket 006)
// owns sourcing them.
func New(routineToken, githubToken string, opts ...Option) *Client {
	c := &Client{
		routineToken:   routineToken,
		githubToken:    githubToken,
		routineBaseURL: defaultRoutineBaseURL,
		httpClient:     http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
