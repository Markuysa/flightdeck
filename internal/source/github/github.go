// Package github implements core.PRReader over the GitHub REST API: open
// pull requests for one owner/repo, keyed by head branch, each carrying its
// CI state derived in ci.go.
//
// It talks to two endpoints with the standard library only (net/http +
// encoding/json) — no google/go-github. CLAUDE.md's stack note lists
// go-git-or-shell for the git source in the same spirit: pull in a large
// dependency tree only when the handful of endpoints actually needed don't
// justify writing them by hand. Two endpoints do not.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
)

// apiBaseURL is GitHub's REST API root. Not configurable: tests stub the
// http.Client's transport instead of pointing the client at a different
// host, so OpenPRs never makes a live network call.
const apiBaseURL = "https://api.github.com"

// ErrGitHubUnavailable wraps any failure reaching or parsing the GitHub
// API: a transport error, a non-2xx response, or a malformed body. Callers
// (the derive/api layer) detect it with errors.Is and downgrade PR/CI state
// to "unknown" while the board still renders from git alone — the failure
// policy docs/ARCHITECTURE.md's "External integrations" table specifies for
// GitHub REST.
var ErrGitHubUnavailable = errors.New("github: unavailable")

// Client implements core.PRReader for one owner/repo.
type Client struct {
	owner, repo string
	token       string
	httpClient  *http.Client
}

var _ core.PRReader = (*Client)(nil)

// Option configures a Client constructed by New.
type Option func(*Client)

// WithHTTPClient overrides the http.Client used for requests. Tests pass
// one whose Transport is a stub http.RoundTripper returning canned
// responses, so no test makes a live API call.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New returns a Client reading owner/repo's open PRs with token. token is a
// constructor parameter only — it is never hardcoded here, and the caller
// (the registry, ticket 006, or an environment variable) owns sourcing it.
// The client never logs or otherwise surfaces token outside the
// Authorization header it sends (see get's doc comment).
func New(owner, repo, token string, opts ...Option) *Client {
	c := &Client{owner: owner, repo: repo, token: token, httpClient: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// pullRequest is the subset of GitHub's pull request JSON this reader
// needs: the head branch and SHA (to key the result and look up CI), plus
// the number and URL the frontend links to.
type pullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

// OpenPRs implements core.PRReader. It lists the repo's open pull requests
// and returns them keyed by head branch (e.g. "claude/007-feature"), each
// annotated with its CI state (see ci.go's ciState). On any API failure it
// returns ErrGitHubUnavailable wrapping the cause and no partial map.
func (c *Client) OpenPRs(ctx context.Context) (map[string]core.PRState, error) {
	var prs []pullRequest
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100", apiBaseURL, c.owner, c.repo)
	if err := c.get(ctx, url, &prs); err != nil {
		return nil, fmt.Errorf("listing open PRs for %s/%s: %w", c.owner, c.repo, err)
	}

	states := make(map[string]core.PRState, len(prs))
	for _, pr := range prs {
		ci, err := c.ciState(ctx, pr.Head.SHA)
		if err != nil {
			return nil, fmt.Errorf("CI state for %s/%s PR #%d: %w", c.owner, c.repo, pr.Number, err)
		}
		states[pr.Head.Ref] = core.PRState{
			Number: pr.Number,
			URL:    pr.HTMLURL,
			CI:     ci,
		}
	}
	return states, nil
}

// get performs an authenticated GET against url and decodes the JSON
// response body into out. Any failure — building the request, the
// transport, a non-2xx status, or a malformed body — comes back wrapped in
// ErrGitHubUnavailable.
//
// Token safety: the token is set only on the request's Authorization
// header, never interpolated into url or any error string this method (or
// its callers) builds, so it cannot leak into logs or returned errors.
func (c *Client) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: building request for %s: %w", ErrGitHubUnavailable, url, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: requesting %s: %w", ErrGitHubUnavailable, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%w: %s returned %d: %s", ErrGitHubUnavailable, url, resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: decoding response from %s: %w", ErrGitHubUnavailable, url, err)
	}
	return nil
}
