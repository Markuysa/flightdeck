# Architecture

Scope: FlightDeck v1, per `docs/PRD.md` §5. No code exists yet — everything below is
being introduced.

## Package layout

```
cmd/flightdeck/           CLI entrypoint: serve, version
internal/
  core/                   domain types + every cross-package interface. Imports nothing internal.
  source/                 reads a project: tickets from disk, status derivation
    git/                  git state (branches, merged-ness) via go-git
    github/               PR + CI state via the REST API
  derive/                 the status engine: (tickets + git + PR state) -> derived board
  registry/              registered projects + secrets, SQLite-backed config only
  dispatch/               routine /fire client, autopilot.json read/write
  api/                    REST + SSE over chi; single-token auth
  webui/                  embed.FS wrapper around ui/dist
  app/                    composition root: builds everything from config, owns lifecycle
ui/                       React + Vite + TypeScript + Tailwind, built to ui/dist
```

Dependency rule: feature packages depend on `internal/core` and never on each other; only
`internal/app` wires concrete implementations. This is what makes the tickets
parallelizable — see ADR-004.

## Data model (internal/core)

```go
type Project struct {
    ID       string   // stable slug
    Name     string
    RepoPath string   // local checkout FlightDeck reads
    Remote   string   // "github" | "" (local-only)
    Owner    string   // github owner/repo, when Remote == "github"
    Repo     string
    // secrets (routine token, github token) live in the registry, not here
}

type Ticket struct {
    ID       int
    Title    string
    Role     string   // designer|frontend|backend|qa|dev
    Depends  []int
    Body     string
    Handoff  string    // the ## Handoff section, when present
    // note: no Status field. Status is derived, never parsed as truth.
}

type DerivedStatus string // ready|in_progress|in_review|blocked|needs_attention|done

type BoardTicket struct {
    Ticket
    Status  DerivedStatus
    Branch  string   // claude/NNN-*, when it exists
    PR      *PRState // when in review
}

type PRState struct {
    Number int
    URL    string
    CI     string // pending|green|red|unknown
}
```

### Interfaces (internal/core)

```go
// A Source reads one project's raw state. git and github implement the parts they own;
// derive composes them. Designed so a gitlab source can be added later (PRD §5).
type TicketReader interface { Tickets(ctx context.Context) ([]Ticket, error) }
type GitState interface {
    Branches(ctx context.Context) ([]string, error)
    IsMergedToMain(ctx context.Context, branch string) (bool, error)
    FileOnBranch(ctx context.Context, branch, path string) (string, error)
}
type PRReader interface { OpenPRs(ctx context.Context) (map[string]PRState, error) } // keyed by branch

type Dispatcher interface {
    Fire(ctx context.Context, p Project, ticketID int) (sessionURL string, err error)
    Autopilot(ctx context.Context, p Project) (on bool, err error)
    SetAutopilot(ctx context.Context, p Project, on bool) error
    ApproveMerge(ctx context.Context, p Project, prNumber int) error
}
```

## Status derivation (internal/derive) — the core

Pure function of its inputs, so it is unit-testable against fixtures with no network:

```
derive(tickets, mainStatuses, branches, openPRs) -> []BoardTicket
```

Rules, exactly as the template's `docs/tickets/README.md` defines them:

- `done` — the ticket file on main says `status: done`
- `needs_attention` — a branch exists and the file on that branch says `needs-attention`
- `in_review` — a branch exists, its file says `done`, main still says `todo`; annotate
  with the open PR and its CI state when a PRReader is present
- `in_progress` — a branch exists, its file still says `todo`
- `blocked` — no branch, and some `depends` id is not `done` on main
- `ready` — no branch, todo, all `depends` done

The engine takes the file's `status` field only for the two literal values it is allowed
to carry (`done`, `needs-attention`); everything else is computed. It never trusts a
stored `in_progress`/`ready`/`blocked`.

## API contract (frozen for the UI)

Auth: `Authorization: Bearer <token>` from `FLIGHTDECK_TOKEN`; UI trades it for a session
cookie via `POST /api/session`.

| Method | Path | Notes |
|---|---|---|
| GET | `/api/projects` | all registered projects with status counts (US-1) |
| POST | `/api/projects` | register: `{name, repo_path, github?}` (US-7) |
| DELETE | `/api/projects/{id}` | unregister |
| GET | `/api/projects/{id}/board` | derived board: tickets grouped by status (US-2) |
| GET | `/api/projects/{id}/tickets/{tid}` | ticket detail + dependency handoffs + PR/CI (US-3) |
| POST | `/api/projects/{id}/dispatch` | `{ticket_id}` -> routine /fire, returns `{session_url}` (US-5) |
| GET/PUT | `/api/projects/{id}/autopilot` | read/flip autopilot.json (US-5) |
| POST | `/api/projects/{id}/tickets/{tid}/approve` | merge the ticket's PR, human action only (US-6) |
| GET | `/api/agents` | live agent sessions (US-4) |
| GET | `/api/events` | SSE: board changed, dispatch started, ci changed |

This table is frozen before UI work starts, so backend and frontend proceed in parallel.

## External integrations

| Integration | Auth | Failure policy |
|---|---|---|
| Local git repo | filesystem | a missing/að unreadable repo marks the project degraded, never crashes the board |
| GitHub REST (PR/CI) | token from registry | on error, tickets still render from git; PR/CI shown as `unknown` |
| Routine /fire | per-project bearer token | dispatch failure surfaces to the user with the error; never silently retried |

## Adopted patterns

- ADR-001 — Derive status, never store it
- ADR-002 — Single binary with embedded UI
- ADR-003 — Source interface so GitLab can join later
- ADR-004 — Composition root in internal/app
- ADR-005 — Secrets in a local config store, never in the browser

## Deliberately not built yet

| Not building | Why | When |
|---|---|---|
| Ticket editing / planning in the UI | planning is `/spec` + `/plan`; the UI drives, it does not author | maybe never |
| GitLab source | interface is designed for it; ship GitHub first | v2 |
| Multi-user / hosted | single self-hosted user is the whole audience | maybe never |
| Analytics / burndown | control surface, not a reporting tool | v2 if asked |
