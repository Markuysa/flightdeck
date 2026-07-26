# FlightDeck

Mission control for a team of coding agents: one screen over many projects' file-based
ticket queues. It derives each ticket's status live from git, shows which agents are
working right now, and lets one person dispatch work and approve merges — the CEO console
for a vibe-coding setup. Self-hosted, single binary, no database of its own for anything
that matters.

> A visual version of this guide: **https://claude.ai/code/artifact/954a5db0-13af-419e-ab45-73a3ba6d9dd7**

## What it is

The vibe-coding setup runs coding agents against a file-based ticket queue
(`docs/tickets/*.md`), but driving it means jumping between a terminal `/board`, `claude
agents`, GitHub, and a `curl` command to dispatch. FlightDeck puts all of that in one
place: see every project and its derived ticket statuses, see which agents are working,
dispatch the next ticket (or a specific one), and approve merges — the human gives
direction, the agents execute. See [`docs/PRD.md`](docs/PRD.md) for the full product story.

## The one idea: derive, never store

Every other design decision follows from this. FlightDeck owns **no ticket database**.
On every board read it recomputes each ticket's status from that project's
`docs/tickets/*.md` plus its git branches and PR/CI state — with the exact rules the
agents themselves use. A ticket is `done` because its branch **merged**, not because a
dashboard row says so.

This is deliberate (see [ADR-001](docs/decisions/ADR-001-derive-never-store.md)). A
dashboard that keeps its own copy of status drifts from what the agents actually see, and
a stale control surface is worse than none. The only thing FlightDeck persists is its own
config: which projects are registered, and their secrets — never a ticket's status.

The engine trusts the ticket file's stored `status:` for only two literal values — `done`
and `needs-attention`. Everything else is **computed**. A ticket file that lies
(`status: ready`) while a dependency is unmerged still derives **blocked**.

### The six derived statuses

| Status | Derived when |
|---|---|
| **ready** | No branch yet, the file says `todo`, and every dependency is `done` on main. Dispatchable now. |
| **in_progress** | A `claude/NNN-*` branch exists and its ticket file still says `todo` — an agent is working. |
| **in_review** | The branch's file says `done` but main still says `todo` — annotated with the open PR and its live CI state. |
| **needs_attention** | The branch's file says `needs-attention` — the agent hit something a human must resolve. |
| **blocked** | No branch, and at least one `depends` id is not yet done on main. Waiting on upstream work. |
| **done** | The ticket file **on main** says `done` — the branch merged. Wins over everything else. |

Status is the whole visual language: on the board it reads instantly by colour, and it can
never disagree with the agents because it computes the same function they do.

## How it's built

A single Go binary that serves the API and embeds the built React UI. Feature packages
depend only on `internal/core`; one composition root wires the concrete pieces together
(see [ADR-004](docs/decisions/ADR-004-composition-root.md)), which is what let the whole
thing be built ticket-by-ticket in parallel.

| Package | Responsibility |
|---|---|
| `internal/core` | Domain types and every cross-package interface. Imports nothing internal — the dependency-free root. |
| `internal/source/git` | Reads a project: tickets from `docs/tickets/*.md`, plus branches, merged-ness, and a file's contents on any branch. |
| `internal/source/github` | Open PRs and CI state via the GitHub REST API (`pending·green·red·unknown`); on error the board still renders from git alone. |
| `internal/derive` | The status engine — a pure, total function turning tickets + git + PRs into the derived board. The heart of the product. |
| `internal/registry` | The *only* thing persisted: registered projects and their secrets, in a small SQLite file. Never ticket status. |
| `internal/dispatch` | The write side: fire a routine, read/flip `.claude/autopilot.json`, approve a merge. Only on explicit request. |
| `internal/api` | REST + SSE over chi. A single bearer token is traded for an httpOnly session cookie; a broker streams live events. |
| `internal/app` | The composition root: builds everything from config, owns the lifecycle, and runs the background board refresher. |
| `internal/webui` | Embeds the built UI (`ui/dist`) so it ships inside the one binary — no external request at runtime. |
| `ui/` | React + Vite + TypeScript + Tailwind. Dark-only. Design tokens come from one place; status colour is the visual language. |

**The read path.** When you open a board: the browser calls `GET /api/projects/{id}/board`
with its session cookie → the API composes `registry` (the project) + `source/git`
(tickets, branches, file-on-branch) + `source/github` (open PRs, CI) → `derive.Derive`
turns that into the board grouped by status → JSON back to the UI. Nothing is cached; it's
computed fresh every request. A background refresher re-derives each project on an interval
and pushes `board.changed` / `ci.changed` over SSE (`GET /api/events`), so the screens
update on their own when a branch merges or CI flips.

## The write side & the autopilot loop

FlightDeck dispatches and merges only on **explicit human action** — there is no
auto-anything on the server itself. Three write actions, each behind a button:

- **Dispatch** — fire a ready ticket: POSTs to the routine's `/fire` endpoint with the
  ticket id and returns a `session_url` (the live agent session you can open).
- **Autopilot** — flip the switch: reads/writes `.claude/autopilot.json` in the project
  repo, preserving its other fields. Autopilot itself lives in the routines, not here.
- **Approve merge** — human sign-off: squash-merges only the named ticket's PR via GitHub.
  Dispatch and merge are separate code paths; neither implies the other.

The loop FlightDeck sits on top of: a human **dispatches** a ready ticket → the routine
spawns a cloud agent → it implements on a `claude/NNN-*` branch → opens a PR → **CI gates**
it → the PR merges (a human approves, or autopilot auto-merges) → the merge unblocks the
next ticket. FlightDeck is the human's window into that loop; the CI gate is what makes
unattended autopilot safe.

## The four screens

| Screen | Route | Answers |
|---|---|---|
| **Fleet** | `/` | Every registered project as a card: per-status counts, a live-agent dot, the autopilot toggle, manage-tokens, register. "Where does everything stand?" |
| **Board** | `/p/:id` | One project as a kanban — six colour-coded lanes, cards with id/title/role and a dependency dot-trail. "What's the state of this queue?" |
| **Ticket** | `/p/:id/t/:tid` | Body, acceptance criteria, dependency trail, upstream handoffs, PR/CI when in review — and the Dispatch / Approve-merge actions. |
| **Agents** | `/agents` | Live sessions across all projects: who's working, on which ticket, last activity, with a link to the running session. |

## Build & run

```sh
# 1. build the UI and copy it into the embed directory
cd ui && npm ci && npm run build && cd ..
cp -r ui/dist/. internal/webui/dist/

# 2. build the single binary
go build -o bin/flightdeck ./cmd/flightdeck

# 3. run it (a token is required)
FLIGHTDECK_TOKEN=<random-secret> ./bin/flightdeck serve
# dev loop instead: FLIGHTDECK_TOKEN=<secret> go run ./cmd/flightdeck serve
```

`flightdeck version` prints the version.

### Zero-setup: demo mode

No real repository to point it at yet? Run:

```sh
./bin/flightdeck serve --demo
```

This seeds a small fixture project with tickets across every derived status — built from a
throwaway local git repository, no network or real remote required. In demo mode the token
and DB path default to dev values, and the token FlightDeck picked is printed on startup
(`flightdeck-demo-token`) so you can sign in.

### Environment

| Var | Default | Notes |
|---|---|---|
| `FLIGHTDECK_TOKEN` | *(none — required)* | The bearer token users trade for a session. Startup fails fast if empty. |
| `FLIGHTDECK_ADDR` | `:8080` | API and UI share this one port. |
| `FLIGHTDECK_DB` | `flightdeck.db` | SQLite registry file (projects + secrets); gitignored. |
| `FLIGHTDECK_REFRESH_INTERVAL` | `5s` | How often the board refresher polls. `off` (or `<= 0`) disables it. |

## Use it

1. **Sign in.** Open `http://localhost:8080` and enter the `FLIGHTDECK_TOKEN` on the login
   screen — it's traded once for a session cookie.
2. **Register a project.** Give it a name and a `repo_path` — a **local checkout** whose
   `docs/tickets/` FlightDeck reads. Optionally add a GitHub `owner/repo` (for PR/CI state)
   and its routine / GitHub tokens; without a remote the board still renders from git alone.
3. **Read the Fleet.** Each card shows the project's status mix, whether an agent is live,
   and the autopilot state. Flip autopilot or manage tokens right there.
4. **Open the Board.** Click a project to see its six lanes — colour tells you at a glance
   what's ready, working, waiting, or needs you.
5. **Act on a ticket.** Click a card: read its criteria and upstream handoffs, then
   **Dispatch** it if it's ready, or **Approve-merge** its PR if it's in review.
6. **Watch the Agents view.** Live sessions appear as work happens; the board refreshes
   itself as branches merge and CI turns green.

## API

A single frozen contract the UI is built against. Auth is a bearer token traded for a
session cookie via `POST /api/session`; every other route needs the cookie (or the bearer).

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/session` | Trade the bearer token for an httpOnly session cookie. |
| `GET` | `/api/projects` | All registered projects with per-status counts, autopilot, live-agent flag. |
| `POST` | `/api/projects` | Register a project (optionally with its routine/GitHub tokens). |
| `DELETE` | `/api/projects/{id}` | Unregister a project. |
| `GET`/`PUT` | `/api/projects/{id}/secrets` | Set tokens, or read only whether each is set — never the values. |
| `GET` | `/api/projects/{id}/board` | The derived board, tickets grouped by status. Computed per request. |
| `GET` | `/api/projects/{id}/tickets/{tid}` | Ticket detail: body, dependency trail, upstream handoffs, PR/CI. |
| `POST` | `/api/projects/{id}/dispatch` | Fire a ready ticket → returns its `session_url`. `409` if not ready. |
| `GET`/`PUT` | `/api/projects/{id}/autopilot` | Read or flip the project's autopilot switch. |
| `POST` | `/api/projects/{id}/tickets/{tid}/approve` | Merge that ticket's PR — human action only. |
| `GET` | `/api/agents` | Live agent sessions across all projects. |
| `GET` | `/api/events` | SSE stream: `board.changed`, `dispatch.started`, `ci.changed`. |

## Security

Routine and GitHub tokens live only in the local SQLite registry, in a table separate from
the project itself. They're read server-side to fire dispatches and read PR/CI state, and
are **never sent to the browser, never logged, never committed** (see
[ADR-005](docs/decisions/ADR-005-secrets-store.md)). `core.Project`, the DTO every
project-listing endpoint returns, carries no token field by construction.

## Screenshots

![Fleet view: every registered project with its derived ticket counts](docs/images/fleet.png)
*Fleet — every registered project at a glance, ticket counts by derived status.*

![Board view: one project's tickets as a kanban by derived status](docs/images/board.png)
*Board — one project's queue as a kanban, columns computed live from git.*

## More

- [`docs/PRD.md`](docs/PRD.md) — product requirements
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — package layout, derivation rules, the
  frozen API contract
- [`docs/DESIGN.md`](docs/DESIGN.md) — the design system the UI is built from
- [`docs/decisions/`](docs/decisions/) — the architecture decision records (ADRs)
