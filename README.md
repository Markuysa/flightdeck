# FlightDeck

Mission control for a team of coding agents. Describe what you want built; FlightDeck
decomposes it into a dependency-ordered ticket queue, dispatches those tickets to Claude
routines as their dependencies land, and shows you the whole thing on one screen — derived
live from git, never from a database of its own. Self-hosted, single binary.

## What it does

You give it a goal. It plans the work into tickets, and — once you flip autopilot — starts
them itself, respecting the dependency graph and a parallelism cap, retrying what stalls
and leaving for you what it can't resolve. Every ticket's status is recomputed from git on
every read, so the board can never disagree with what the agents actually see.

The human decides *what* to build, *who* builds it, and *what lands on main*. Everything
between those is automatic.

```
      you                    FlightDeck                     the routine
       │                          │                              │
  "add billing"  ──plan──▶  ticket graph                         │
       │                          │                              │
    review ─────apply──▶  docs/tickets/*.md                      │
       │                          │                              │
  autopilot ON ──────▶  scheduler picks `ready` ────fire────▶  agent works
       │                          │                              │
       │                   board updates  ◀────branch, PR────────┘
       │                          │
  approve merge ─────────────────▶│  (or the routine's own auto-merge, behind CI)
       │                          │
       └──────────  merge unblocks the next tickets  ◀───────────┘
```

## Quick start (no setup)

```sh
make            # build the UI, embed it, build the binary
./bin/flightdeck serve --demo
```

Open `http://localhost:8080`, sign in with `flightdeck-demo-token` (printed on startup).
Demo mode seeds a throwaway git repository with tickets across every derived status — no
network, no real remote, nothing to configure. Good for seeing the shape of it before
wiring a real project.

`flightdeck version` prints the version.

---

## Working with a real project

### 1. What you need first

FlightDeck orchestrates; it does not run agents itself. You need a **Claude routine** that
implements a ticket, on the other end of a trigger. See
[the routine contract](#what-your-routine-receives) for exactly what it will be sent.

You also need a local checkout of the project. FlightDeck reads `docs/tickets/*.md` and git
state from the working tree — it never clones for you.

### 2. Run it

```sh
FLIGHTDECK_TOKEN=$(openssl rand -hex 32) ./bin/flightdeck serve
```

Sign in with that token; it is traded once for an httpOnly session cookie.

### 3. Register the project

**Register project** on the Fleet screen. Fields:

| Field | Required | What it's for |
|---|---|---|
| Name | ✅ | Display name; its slug becomes the project id |
| Repository path | ✅ | Local checkout whose `docs/tickets/` is read |
| Routine trigger ID | — | The Claude routine that implements this project's tickets. **Without it the board renders fine but dispatch answers 409** |
| GitHub owner / repo | — | Enables PR and CI state on the board |
| Routine token | — | Bearer token for the trigger call |
| GitHub token | — | For reading PRs and merging |

Tokens are stored server-side in SQLite and never sent back to the browser
([ADR-005](docs/decisions/ADR-005-secrets-store.md)).

### 4. Plan the work

**Plan work** (on the project card or the board) → describe the goal in plain language →
review what comes back → **Write N tickets**.

Planning needs `ANTHROPIC_API_KEY` in the server's environment. Without it the screen
reports 501 and everything else keeps working — planning is the one feature that costs
money per use, and merely running the binary should not switch it on.

Two things about the review step:

- **Proposing writes nothing.** You see every ticket's body, acceptance criteria and
  dependencies before anything touches the repository. That matters because applied
  tickets are what the scheduler then dispatches — a bad decomposition approved in one
  click is a lot of agent time spent wrongly.
- **Applying writes files, and stops there.** Tickets land in `docs/tickets/` with ids
  continuing from whatever is already queued. Nothing is committed and nothing is
  dispatched. Commit them yourself when you're happy.

The plan is validated before it can be applied: cycles, self-dependencies, dangling
`depends` ids and unknown roles are all rejected. This is not pedantry — `blocked` is
derived from `depends` on every read, so a bad edge produces no error anywhere, just a
ticket that can never become ready.

### 5. Staff the agents

**Agents** (on the card or the board) → configure one specialist per role.

| Field | What it does |
|---|---|
| Name | Shown on the board and sent to the routine |
| System prompt | Sent with **every** ticket this agent takes |
| Skills | Comma-separated, passed to the routine verbatim |

One agent per role per project. A role with nobody configured still dispatches — it just
carries no extra instructions, exactly as it did before agents existed.

Prompts are per-project because they have to be: "follow the repo's conventions" means
nothing across two repositories.

### 6. Dispatch — by hand, or let it run

**By hand:** open a `ready` ticket → **Dispatch**. You can add a per-dispatch note ("use
the v2 endpoint, not v1") that rides along with the agent's prompt.

**Autonomously:** flip **Autopilot** on the project card and start the server with
`FLIGHTDECK_SCHEDULE_INTERVAL` set. The scheduler then picks up `ready` tickets itself, up
to `FLIGHTDECK_MAX_PARALLEL` in flight per project.

> ⚠️ **Size this before you enable it.** `MAX_PARALLEL` × projects with autopilot on = how
> many agent sessions can run at once with nobody watching. There is no budget ceiling —
> the parallelism cap is the only bound on spend. Start at 1, on one project.

### 7. Land it

A finished agent opens a PR. **Approve merge** squash-merges it — or the routine's own
autopilot does, behind a CI gate. The merge flips the ticket to `done` on main, which
unblocks everything that depended on it, which the scheduler picks up on its next tick.

**The scheduler never merges.** Starting work an agent will open a PR for is recoverable;
landing code on main is not, so that step keeps its gate
([ADR-007](docs/decisions/ADR-007-autonomous-scheduler.md)).

---

## What your routine receives

Dispatch is `POST {ANTHROPIC_API}/v1/code/triggers/{trigger_id}/run` with the project's
routine token as a bearer, and this body:

```json
{
  "ticket_id": 5,
  "agent_name": "Frontend specialist",
  "agent_prompt": "Match the existing design tokens. Never introduce a new colour.",
  "agent_skills": ["react", "tailwind", "vitest"],
  "ticket_notes": "Use the v2 endpoint, not v1."
}
```

Every field except `ticket_id` is omitted when empty, so a project with no agents
configured sends `{"ticket_id": 5}` — byte-identical to what FlightDeck sent before agents
existed. **An existing routine keeps working untouched;** one that wants the briefing has
to be updated to read those fields.

FlightDeck states who is working and under what instructions. Acting on that is your
routine's prompt.

Your routine is expected to:

1. Read `docs/tickets/NNN-*.md` for the ticket id it was given.
2. Work on a branch named `claude/NNN-slug` — the branch name is how FlightDeck knows an
   agent picked the work up.
3. Set the ticket file's `status:` to `done` on that branch when finished, or
   `needs-attention` when it hits something a human must resolve.
4. Open a PR.

The response should carry a link to the live session. FlightDeck accepts `session_url`,
`url`, `routine_url`, `web_url` or `session.url` — the exact spelling of the
remote-trigger API's field is not pinned down by this codebase
([ADR-006](docs/decisions/ADR-006-routine-trigger.md)), so any of them work, and a response
carrying none is a successful dispatch with no link, not an error.

---

## The one idea: derive, never store

FlightDeck owns **no ticket database**. On every board read it recomputes each ticket's
status from that project's `docs/tickets/*.md` plus its git branches and PR/CI state — with
the exact rules the agents themselves use. A ticket is `done` because its branch
**merged**, not because a dashboard row says so.

This is deliberate ([ADR-001](docs/decisions/ADR-001-derive-never-store.md)). A dashboard
that keeps its own copy of status drifts from what the agents actually see, and a stale
control surface is worse than none.

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

### What *is* persisted

Three things, all facts about FlightDeck's own actions rather than derived state:

- **Registered projects and their secrets** — the configuration.
- **Agent configuration** — prompts and skills exist nowhere in git.
- **Run history** — "this server fired routine R for ticket 7 at 12:04, and the branch
  never appeared." Unavailable from git by construction, and load-bearing: firing does not
  change the board, so without it the scheduler would re-fire the same ticket every tick
  until its branch showed up.

None of these is a ticket status. A test fails the build if any column in the schema ever
contains the word.

---

## Environment

| Var | Default | Notes |
|---|---|---|
| `FLIGHTDECK_TOKEN` | *(required)* | Bearer token users trade for a session. Startup fails fast if empty. |
| `FLIGHTDECK_ADDR` | `:8080` | API and UI share one port. |
| `FLIGHTDECK_DB` | `flightdeck.db` | SQLite file: projects, secrets, agents, runs. Gitignored. |
| `FLIGHTDECK_REFRESH_INTERVAL` | `5s` | How often the board refresher polls. `off` disables it. |
| `ANTHROPIC_API_KEY` | *(none)* | Enables planning. Absent → the planning routes report 501. |
| `FLIGHTDECK_ROUTINE_API_BASE` | `https://api.claude.ai/api` | Remote-trigger API root. Configurable because the host was read off the documented shape rather than verified from here. |
| **Scheduler** | | |
| `FLIGHTDECK_SCHEDULE_INTERVAL` | **disabled** | How often to look for work to start. Unset, empty, `off` or `<= 0` all mean off. |
| `FLIGHTDECK_MAX_PARALLEL` | `2` | Tickets in flight per project — counting both `in_progress` tickets and fired runs whose branch hasn't appeared yet. |
| `FLIGHTDECK_RUN_TIMEOUT` | `30m` | How long a fired run may go without its branch appearing before it's retried. |
| `FLIGHTDECK_MAX_ATTEMPTS` | `2` | Dispatches per ticket before the scheduler gives up and leaves it for a human. |

`FLIGHTDECK_SCHEDULE_INTERVAL` is the one variable where unset does **not** mean "use a
sensible default". Observing is free; dispatching is not, so a server that starts work on
its own has to be asked to. Every scheduler value fails startup if it doesn't parse — a
typo must never silently fall back to a default that spends money.

---

## The screens

| Screen | Route | Answers |
|---|---|---|
| **Fleet** | `/` | Every project as a card with a queue-composition bar, plus fleet totals: how much is queued, working, and waiting on you. |
| **Board** | `/p/:id` | One project as a kanban — six colour-coded lanes, role filters, and the dispatch history below. |
| **Ticket** | `/p/:id/t/:tid` | Body, acceptance criteria, dependency trail, upstream handoffs, PR/CI — and Dispatch / Approve-merge. |
| **Plan** | `/p/:id/plan` | Describe a goal, review the proposed tickets, write them. |
| **Agents** | `/p/:id/agents` | One specialist per role: prompt and skills. |
| **Live agents** | `/agents` | Sessions working right now across every project, with links to them. |

The sidebar carries a **Live / Offline** indicator. The board updates itself only while the
event stream is open, and a disconnected screen looks identical to an up-to-date one — so
the connection state is shown rather than merely handled.

---

## API

Auth is a bearer token traded for a session cookie via `POST /api/session`; every other
route needs the cookie (or the bearer).

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/session` | Trade the bearer token for an httpOnly session cookie. |
| `GET` `POST` | `/api/projects` | List with per-status counts / register a project. |
| `DELETE` | `/api/projects/{id}` | Unregister. Drops its secrets, agents and run history too. |
| `GET` `PUT` | `/api/projects/{id}/secrets` | Set tokens, or read only *whether* each is set. |
| `GET` | `/api/projects/{id}/board` | The derived board, grouped by status. Computed per request. |
| `GET` | `/api/projects/{id}/tickets/{tid}` | Ticket detail with dependency trail and upstream handoffs. |
| `POST` | `/api/projects/{id}/dispatch` | Fire a ready ticket → `session_url`. `409` if not ready, or if no routine trigger is configured. |
| `GET` `PUT` | `/api/projects/{id}/autopilot` | Read or flip the autopilot switch. |
| `POST` | `/api/projects/{id}/tickets/{tid}/approve` | Squash-merge that ticket's PR. |
| `POST` | `/api/projects/{id}/plan` | Decompose a goal → a plan. Writes nothing. `501` without an API key, `422` if the graph is invalid. |
| `POST` | `/api/projects/{id}/plan/apply` | Write an approved plan's tickets. Revalidates first. |
| `GET` `POST` | `/api/projects/{id}/agents` | List / configure agents for this project. |
| `DELETE` | `/api/projects/{id}/agents/{agentID}` | Remove one. |
| `GET` | `/api/projects/{id}/runs` | Dispatch history, most recent first. |
| `GET` | `/api/agents` | Live agent sessions across all projects. |
| `GET` | `/api/events` | SSE: `board.changed`, `dispatch.started`, `ci.changed`. |

---

## How it's built

A single Go binary serving the API and embedding the built React UI. Feature packages
depend only on `internal/core`; one composition root wires the concrete pieces
([ADR-004](docs/decisions/ADR-004-composition-root.md)).

| Package | Responsibility |
|---|---|
| `internal/core` | Domain types and every cross-package interface. Imports nothing internal. |
| `internal/source/git` | Tickets from `docs/tickets/*.md`, branches, merged-ness, a file's contents on any branch. |
| `internal/source/github` | Open PRs and CI state. ETag-revalidated and rate-limit aware; on error the board still renders from git alone. |
| `internal/derive` | The status engine — a pure, total function turning tickets + git + PRs into the board. |
| `internal/plan` | Decomposes a goal into a validated ticket graph, and writes an approved one to disk. |
| `internal/schedule` | The autonomous loop: reads each board, dispatches `ready` tickets within its caps. Never merges. |
| `internal/registry` | What is persisted: projects, secrets, agents, run history. Never ticket status. |
| `internal/dispatch` | Fire a routine trigger, read/flip `.claude/autopilot.json`, approve a merge. |
| `internal/api` | REST + SSE over chi, with a broker streaming live events. |
| `internal/app` | Composition root: builds everything from config, owns the lifecycle, runs the refresher and the scheduler. |
| `internal/webui` | Embeds `ui/dist` so it ships inside the one binary. |
| `ui/` | React + Vite + TypeScript + Tailwind. Dark-only; every colour comes from one token file. |

```sh
make          # ui + binary
make test     # Go + UI suites
make lint     # go vet + token check + eslint
make demo     # serve the seeded fixture project
make help     # everything else
```

Use `make`, not `go test ./...` — the UI's `node_modules` contains a stray Go package that
`./...` picks up.

---

## Security

Routine and GitHub tokens live only in the local SQLite registry, in a table separate from
the project itself. They're read server-side and are **never sent to the browser, never
logged, never committed** ([ADR-005](docs/decisions/ADR-005-secrets-store.md)).
`core.Project`, the DTO every project-listing endpoint returns, carries no token field by
construction.

The routine trigger id is *not* a secret — it names which routine to run, and the bearer
token is what authorizes the call — so it rides on the project DTO like any other field.

---

## Known limits

Worth knowing before you rely on it:

- **No budget ceiling.** Token spend and agent logs live on the routine's side with no
  callback to this server, so FlightDeck cannot report cost or stream logs. The
  parallelism cap is the only bound on autonomous spend.
- **Parallel tickets can conflict.** The dependency graph orders what must be ordered, but
  knows nothing about which files a ticket touches. Two agents editing the same file
  produce a merge conflict on the second PR. Mitigated by the parallelism cap and by
  decomposition quality; not solved. It's why `MAX_PARALLEL` defaults to 2.
- **Ticket ids are assigned from the filesystem at apply time**, so two people applying
  plans to one project simultaneously can collide. Apply refuses to overwrite rather than
  clobbering, so the failure is loud.
- **Agent activity is inferred from branch tip commit time.** An agent thinking for twenty
  minutes without committing looks idle.

## More

- [`docs/AUDIT.md`](docs/AUDIT.md) — a full assessment of what's built and what isn't
- [`docs/PRD.md`](docs/PRD.md) — product requirements
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — package layout, derivation rules, the API contract
- [`docs/DESIGN.md`](docs/DESIGN.md) — the design system the UI is built from
- [`docs/decisions/`](docs/decisions/) — the architecture decision records
