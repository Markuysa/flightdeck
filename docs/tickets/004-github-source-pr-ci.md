---
id: 4
title: GitHub source — PR and CI state
role: backend
depends: [1]
status: done
---
Implement `core.PRReader` over the GitHub REST API: open PRs keyed by head branch, each
with number, URL, and derived CI state.

## Likely files
- `internal/source/github/{github,ci}.go`

## Acceptance criteria
- [ ] `OpenPRs` returns a map keyed by head branch → `PRState{Number,URL,CI}`.
- [ ] CI state derived from check runs into `pending|green|red|unknown`; missing checks is
      `unknown`, not an error.
- [ ] The token comes from the registry/env, never hardcoded; never logged.
- [ ] HTTP client is injectable; tests run against a stubbed transport with recorded
      responses — no live API call in tests.
- [ ] On any API error the reader returns a typed error the caller can downgrade to
      `unknown`, so the board still renders from git alone.
- [ ] lint + tests pass.

## Handoff

Package `internal/source/github` implements `core.PRReader`. Consumers: ticket 005 (derive
annotates in-review tickets with PR + CI), 006 (registry supplies the token), 008 (api
composes it).

**Constructor:** `New(owner, repo, token string, opts ...Option) *Client`, with
`WithHTTPClient(*http.Client) Option` for injection. The token is set only on the request's
`Authorization: Bearer <token>` header — never in a URL, log, or error string (a test
enforces this). Ticket 006's registry provides the per-project token; pass `""` for a
public/unauthenticated read.

**`OpenPRs(ctx) (map[string]core.PRState, error)`** — keyed by **PR head branch name**
(e.g. `"claude/007-dispatch-fire-autopilot-approve"`), value
`core.PRState{Number, URL (html_url), CI}`. Endpoint: check-runs for the PR head SHA
(`GET /repos/{owner}/{repo}/commits/{sha}/check-runs`); the legacy commit-status API is not
queried (documented in `ci.go` — a PR with only legacy statuses reads `unknown`).

**CI-state mapping — what the frontend colours by (`PRState.CI`):**

| `CI` value | Meaning | Rule (precedence, top wins) | DESIGN.md colour to use |
|---|---|---|---|
| `red` | a check failed | any check-run conclusion is `failure`, `timed_out`, `cancelled`, or `action_required` | `--st-attention` (needs a human) |
| `pending` | checks still running | else any check-run `status` ≠ `completed` (`queued`/`in_progress`) | `--st-progress` (working) |
| `green` | all checks passed | else every check-run `completed` and none red (`success`/`neutral`/`skipped`) | `--st-done` (merged/ok) |
| `unknown` | no CI info | zero check-runs recorded for the SHA **(not an error)** | `--text-dim` (quiet) |

The four strings are exactly what the Go type emits; the frontend already types `PRState.CI`
as `pending|green|red|unknown` in `ui/src/lib/types.ts`. Map them to colours in the UI, do
not re-derive.

**Failure policy (matches ARCHITECTURE.md "External integrations"):** on any API/network/decode
error `OpenPRs` returns a wrapped `ErrGitHubUnavailable`. Callers should
`errors.Is(err, github.ErrGitHubUnavailable)` and **downgrade to rendering from git alone,
showing CI as `unknown`** — the board never fails because GitHub is down. `errors.Is` also
matches the underlying cause (the error is double-wrapped with `%w`).
