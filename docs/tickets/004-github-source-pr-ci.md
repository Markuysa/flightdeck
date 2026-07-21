---
id: 4
title: GitHub source — PR and CI state
role: backend
depends: [1]
status: todo
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
State the CI-state mapping so the frontend renders the right colour.
