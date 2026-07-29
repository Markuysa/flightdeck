# ADR-008: Planning proposes, agents brief, observability stops at the routine

Date: 2026-07-27
Status: accepted

## Context
Three things the audit found missing were all requested together: decompose a goal into
tickets with a model, configure per-role agents with prompts and skills, and see what the
system has actually been doing. They are one decision because they share a boundary — how
much of the agent's behaviour FlightDeck owns versus the routine.

## Decision

**Planning proposes; a separate call writes.** `POST /plan` returns a graph for review and
touches nothing. `POST /plan/apply` writes `docs/tickets/*.md`. The split is the safety
property: applied tickets are what the autonomous scheduler then dispatches, so a model that
could reach the filesystem directly could redirect every agent that runs afterwards. The plan
round-trips through the browser, so an operator's edits are what land — and the server
revalidates on apply, because trusting the returned plan would let a hand-edited cycle through.

**The graph is validated, not just shape-checked.** Structured outputs guarantee the JSON
shape; nothing guarantees the graph. A cycle or a dangling `depends` produces no error
anywhere downstream — `blocked` is derived from `depends` on every read, so a bad edge yields
tickets that can never become ready and a queue that silently never starts. `internal/plan`
rejects cycles, self-dependencies, dangling ids, unknown roles, and plans over the ticket cap.

**Apply never commits and never renumbers.** Writing the files is what makes them visible to
the derive engine; committing is the operator's call, and a planning API that pushes to a
branch nobody asked about is not one. New ids continue from the highest already present, so
applying can never rewrite work already queued or in flight.

**Agents are per-role, one per role, per project.** A ticket's `role` frontmatter is what
binds it to an agent, so dispatch looks an agent up *by role* — two agents sharing one would
make which prompt an agent receives depend on row order. Prompts are per-project because
"follow the repo's conventions" means nothing across two repos.

**Skills are passed through, not validated.** Which skills exist is the routine's business; a
list hardcoded here would go stale the moment the routine gained one.

**A briefing is decoration, never a preconditon.** An unstaffed role, an unreadable agent
store, an empty project — all dispatch exactly as they did before agents existed. Failing a
dispatch because the *instructions* could not be loaded would trade a working feature for a
broken one. Both dispatch paths (human and scheduler) build the same briefing, so a ticket
started either way reaches the routine with identical instructions.

**Run history is the observability that is actually available.** The board shows current state
derived from git; a dispatch that failed or timed out before its agent pushed a branch leaves
no trace there at all. `GET /runs` is the only place those attempts exist.

## Consequences
- The product does the thing it was described as doing: a goal becomes a reviewed ticket
  graph, and the scheduler drains it with role-appropriate specialists.
- **Planning is off unless `ANTHROPIC_API_KEY` is set** — the routes report 501 and every
  other route works. Deliberate: it is the one feature that costs money per use, and merely
  running the binary should not switch it on.
- **The remote-trigger body grew four optional fields** (`agent_name`, `agent_prompt`,
  `agent_skills`, `ticket_notes`). They are `omitempty`, so a project with no agents sends
  byte-identical requests to before and an existing routine keeps working untouched. A routine
  that wants the briefing has to be updated to read them — FlightDeck states who is working
  and under what instructions; acting on that is the routine's prompt.
- **Cost and log streaming remain unbuilt, and cannot be built from here.** FlightDeck fires a
  trigger and gets a session URL back; token usage and agent logs live on the routine's side
  with no callback to this server. Audit items 4.1 and 4.2 stay open, and they are blocked on
  a routine-side webhook, not on effort here. The practical consequence: **there is still no
  budget ceiling** — the parallelism cap is the only bound on autonomous spend.
- Ticket ids are assigned at apply time from the filesystem, so two operators applying plans
  to one project concurrently can collide. Apply refuses to overwrite rather than clobbering,
  so the failure is loud, but it is a rough edge worth closing when it bites.
