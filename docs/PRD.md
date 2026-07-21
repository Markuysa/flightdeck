# FlightDeck — Product Requirements

## 1. Problem and goal

The vibe-coding setup runs a team of coding agents against a file-based ticket queue. It
works, but it is driven from a terminal and scattered surfaces: `/board` for the queue,
`claude agents` for live sessions, GitHub for PRs, a curl command to dispatch. There is no
single place where a person can see every project, see what the team is doing, and steer
it.

FlightDeck is that place: one screen where the human — the CEO of the agent team — sees
all projects and their tickets, sees which agents are working right now, dispatches the
next work, and approves merges. It gives direction; the team executes.

## 2. Audience

One user: the person running the agent team (may register several projects). Not
multi-tenant, not a hosted SaaS. Self-hosted single binary, same philosophy as the
projects it manages.

## 3. Core principle: derive, never store

FlightDeck owns no ticket database. For each registered project it reads `docs/tickets/*.md`
and git state, and derives status the same way the agents do (`docs/tickets/README.md` in
the template). A ticket is `done` because its branch merged, not because a dashboard row
says so. This is the product's defining constraint: the board cannot disagree with reality
because it computes reality on every read.

## 4. User stories

- **US-1** As the CEO, I see all my projects on one board, each showing its ticket counts
  by derived status (ready, in progress, in review, blocked, done, needs-attention).
- **US-2** I open a project and see its tickets as a kanban: columns by status, cards
  showing id, title, role, and dependencies.
- **US-3** I open a ticket and see its body, acceptance criteria, dependency chain, the
  handoffs from its dependencies, and — if in review — its PR and CI state.
- **US-4** I see which agents are working right now and on what (live session view).
- **US-5** I dispatch the next ready ticket, or a specific one, with one action —
  triggering the project's routine. I can toggle a project's autopilot on or off.
- **US-6** I review a ticket that is in review and approve the merge, or send it back.
  Merging is always my decision.
- **US-7** I register a new project by pointing at its git repository and (optionally) its
  routine dispatch endpoint.

## 5. Scope for v1

In:
- Multi-project board (US-1) and per-project kanban (US-2, US-3)
- Status derivation engine reading local git + GitHub PR/CI state
- Dispatch via routine `/fire` and autopilot toggle (US-5)
- Merge-approve for in-review tickets (US-6)
- Project registration and the config store (US-7)
- Live agent view (US-4) — read-only embed/link of Claude's Agent View is acceptable for
  v1 if a native view is too large

Explicitly out (later):
- Editing tickets or writing plans from the UI — planning stays in `/spec` and `/plan`
- GitLab support in v1 (design the source interface for it, ship GitHub first)
- Multi-user, auth beyond a single local token, hosted deployment
- Historical analytics, burndown, velocity — this is a control surface, not a reporting tool

## 6. Acceptance criteria (top level; tickets refine these)

- [ ] Registering a local repo with `docs/tickets/` makes its tickets appear with correct
      derived statuses, verified against a fixture repo with known git state.
- [ ] A ticket whose branch is merged shows `done`; one with an open PR shows `in review`
      with its live CI state; one with an unmet dependency shows `blocked`. No status is
      ever read from a stored field.
- [ ] Dispatching a ticket issues the routine `/fire` call and surfaces the returned
      session URL. Dispatch is disabled for a ticket that is not ready.
- [ ] Autopilot toggle reflects and flips `.claude/autopilot.json` in the target repo.
- [ ] Approving a merge merges the PR (GitHub) on human action only; the server never
      merges on its own.
- [ ] Secrets (routine tokens, GitHub token) never reach the browser and never appear in
      logs or the committed config.
- [ ] The UI uses only `docs/DESIGN.md` tokens; a hardcoded colour fails the lint.

## 7. Success criteria

The human runs a real sprint — dispatches, watches, approves merges — entirely from
FlightDeck, without dropping to the terminal for queue or dispatch operations. If they
still need `/board` and curl, it failed.
