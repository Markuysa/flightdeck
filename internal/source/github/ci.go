package github

import (
	"context"
	"fmt"
)

// CI state literals. These are the exact strings core.PRState.CI carries
// and the frontend switches on to pick a colour.
const (
	ciPending = "pending"
	ciGreen   = "green"
	ciRed     = "red"
	ciUnknown = "unknown"
)

// checkRunsResponse is the body of GitHub's "List check runs for a Git
// reference" endpoint.
type checkRunsResponse struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []checkRun `json:"check_runs"`
}

// checkRun is the subset of a single check run this reader inspects.
type checkRun struct {
	Status     string `json:"status"`     // queued | in_progress | completed
	Conclusion string `json:"conclusion"` // success | failure | neutral | cancelled | skipped | timed_out | action_required | stale | "" (not yet concluded)
}

// ciState fetches the check runs recorded against sha and reduces them to
// one of pending|green|red|unknown.
//
// Endpoint: GET /repos/{owner}/{repo}/commits/{sha}/check-runs — the
// "Check Runs" API, which reports the combined status GitHub Actions (and
// every other check-runs-integrated CI system) records against a commit.
// GitHub also has an older "Commit Status" API
// (/commits/{sha}/status) for systems that predate check runs; this reader
// does not call it. Querying both and merging their precedence would double
// the API calls of every OpenPRs call to cover CI systems that predate
// GitHub Actions and are not this project's target, so check runs alone
// keeps the implementation to one endpoint. A PR whose only CI reports
// through the legacy status API falls through the "no check runs" branch
// below and is reported as "unknown" — the same safe default as a PR with
// no CI configured at all, and not an error.
//
// Precedence, applied in this order:
//  1. red     — any check run's conclusion is failure, timed_out,
//     cancelled, or action_required. One red check fails the PR even if
//     others are still running or already passed.
//  2. pending — no red check, but at least one check run has not completed
//     yet (status is queued or in_progress).
//  3. green   — every check run is complete and none of them is red (i.e.
//     all concluded success, neutral, or skipped — or any other completed
//     conclusion GitHub adds later; only the four conclusions above are
//     treated as failing).
//  4. unknown — no check runs are recorded against sha at all. This is the
//     default for a repo/PR with no CI configured and is not an error; the
//     ARCHITECTURE.md failure policy of downgrading to "unknown" applies to
//     API errors, not to the legitimate absence of checks.
func (c *Client) ciState(ctx context.Context, sha string) (string, error) {
	var resp checkRunsResponse
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs", apiBaseURL, c.owner, c.repo, sha)
	if err := c.get(ctx, url, &resp); err != nil {
		return "", err
	}

	if len(resp.CheckRuns) == 0 {
		return ciUnknown, nil
	}

	pending := false
	for _, run := range resp.CheckRuns {
		switch run.Conclusion {
		case "failure", "timed_out", "cancelled", "action_required":
			return ciRed, nil
		}
		if run.Status != "completed" {
			pending = true
		}
	}
	if pending {
		return ciPending, nil
	}
	return ciGreen, nil
}
