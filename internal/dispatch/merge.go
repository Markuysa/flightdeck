package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
)

// githubAPIBaseURL is GitHub's REST API root, matching
// internal/source/github's apiBaseURL. It is not shared code across
// packages (ADR-004 forbids one feature package importing another) and not
// configurable: tests stub the http.Client's transport instead, so
// ApproveMerge never makes a live network call.
const githubAPIBaseURL = "https://api.github.com"

// ErrMergeFailed wraps any failure merging a PR: a transport error, a
// non-2xx response, a malformed body, or GitHub reporting the merge did not
// happen (merged: false) despite a 2xx status.
var ErrMergeFailed = errors.New("dispatch: merge failed")

// mergeRequest is the body ApproveMerge PUTs to GitHub's merge endpoint.
// merge_method is fixed to "squash" (docs/tickets/README.md's documented
// merge step: "gh pr merge --auto --squash").
type mergeRequest struct {
	MergeMethod string `json:"merge_method"`
}

// mergeResponse is the subset of GitHub's merge response this reader
// inspects: whether the merge actually happened, and GitHub's message when
// it did not.
type mergeResponse struct {
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

// ApproveMerge implements core.Dispatcher.ApproveMerge: it squash-merges
// p's pull request prNumber via the GitHub API, using the per-project
// GitHub token, and ONLY when this method is called.
//
// This is the one place in the package that ever issues a merge request.
// Fire (fire.go) never calls ApproveMerge and never issues a request to this
// endpoint — the two methods build entirely separate requests to entirely
// separate URLs (the routine's /fire vs. GitHub's pulls/.../merge), so there
// is no code path where dispatching a ticket implies merging its PR.
// merge_test.go's TestFireAndApproveMergeAreSeparateCodePaths asserts
// this directly against a request-recording transport. This is what
// CLAUDE.md's "dispatches and merges only on explicit human action; no
// auto-anything on the server" requires in code, not just in a doc comment.
func (c *Client) ApproveMerge(ctx context.Context, p core.Project, prNumber int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", githubAPIBaseURL, p.Owner, p.Repo, prNumber)

	body, err := json.Marshal(mergeRequest{MergeMethod: "squash"})
	if err != nil {
		return fmt.Errorf("%w: encoding request: %w", ErrMergeFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: building request: %w", ErrMergeFailed, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: requesting %s: %w", ErrMergeFailed, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%w: %s returned %d: %s", ErrMergeFailed, url, resp.StatusCode, respBody)
	}

	var out mergeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("%w: decoding response from %s: %w", ErrMergeFailed, url, err)
	}
	if !out.Merged {
		return fmt.Errorf("%w: PR #%d for %s/%s: %s", ErrMergeFailed, prNumber, p.Owner, p.Repo, out.Message)
	}
	return nil
}
