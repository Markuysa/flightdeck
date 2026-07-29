// Package dispatch implements core.Dispatcher: firing a project's routine,
// reading and flipping its autopilot switch, and approving a PR merge.
//
// Every method here is a plain action with no policy of its own — it does
// what it is told, once, and never retries. WHO is allowed to call it, and
// when, is decided above: a human through internal/api's handlers, or the
// autonomous scheduler in internal/schedule (ADR-007). The one line that has
// not moved is that **ApproveMerge is never called by the scheduler**:
// starting work is recoverable, landing it on main is not.
//
// A Client is constructed once per project with that project's routine and
// GitHub tokens (sourced from the registry, ticket 006); the owner/repo,
// local repo path and routine trigger id a given call needs come from the
// core.Project argument each method already takes, so one Client is safe to
// reuse across calls for the same project.
//
// Dispatch runs a Claude routine: the project names a trigger
// (core.Project.RoutineTriggerID), Fire runs it through the claude.ai
// remote-trigger API, and the routine takes it from there — implementing
// the ticket on a claude/NNN-* branch and opening a PR. FlightDeck's part
// ends at "started"; it never drives the agent. Swapping routines for
// another execution backend means replacing this package alone — nothing
// outside it names a trigger endpoint, because core.Dispatcher is the seam
// (see docs/decisions/ADR-006-routine-trigger.md).
//
// Fire (fire.go) and ApproveMerge (merge.go) are two independent methods
// that never call each other or share a code path: Fire only ever issues a
// POST to the trigger's /run endpoint, ApproveMerge only ever issues a PUT
// to GitHub's merge endpoint. See merge.go's doc comment for the invariant
// this encodes and merge_test.go's separation test for the proof.
package dispatch

import (
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
)

// DefaultRoutineAPIBase is the claude.ai remote-trigger API root Fire posts
// to when the caller does not override it with WithRoutineAPIBase. Fire
// appends "/v1/code/triggers/{trigger_id}/run" to it.
//
// It is a var, not a const, and is surfaced as FLIGHTDECK_ROUTINE_API_BASE
// by the composition root, for one honest reason: this host and path prefix
// were read off the remote-trigger API's documented shape, not verified
// against a live call from this codebase. If Anthropic serves it from a
// different host, an operator changes one environment variable instead of
// waiting for a rebuild. See docs/decisions/ADR-006-routine-trigger.md.
var DefaultRoutineAPIBase = "https://api.claude.ai/api"

// Client implements core.Dispatcher for one project's routine and GitHub
// tokens.
type Client struct {
	routineToken   string
	githubToken    string
	routineAPIBase string
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

// WithRoutineAPIBase overrides the remote-trigger API root (e.g.
// "https://api.claude.ai/api"); Fire POSTs
// "<base>/v1/code/triggers/{trigger_id}/run". The composition root sets
// this from FLIGHTDECK_ROUTINE_API_BASE; tests point it at an httptest
// server.
func WithRoutineAPIBase(url string) Option {
	return func(c *Client) { c.routineAPIBase = url }
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
		routineAPIBase: DefaultRoutineAPIBase,
		httpClient:     http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
