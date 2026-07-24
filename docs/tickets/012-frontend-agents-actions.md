---
id: 12
title: Frontend — Agents live view
role: frontend
depends: [2]
status: done
---
Live agent sessions (US-4): who is working, on which ticket, last activity — the "team at
work" view.

## Likely files
- `ui/src/pages/Agents.tsx`, `ui/src/components/AgentRow.tsx`

## Acceptance criteria
- [ ] Lists sessions from `GET /api/agents`, each row: agent/role, project, ticket, a
      pulsing live dot when active (the only motion in the app, §2.4), last-activity time.
- [ ] Updates live via SSE; a session going idle stops pulsing.
- [ ] Empty state when no agents are working. Tokens only; tested against a mocked stream.
      lint + build + test pass.

## Handoff

The Agents screen (`ui/src/pages/Agents.tsx`) is real — the last of the four screens. With
009–012 merged, every route in the Shell now renders a real view, so ticket 013 can wire the
binary and embed `ui/dist` against a complete UI.

**`ui/src/lib/agentActivity.ts` (pure, reusable):**
```ts
export const ACTIVE_THRESHOLD_MS = 5 * 60 * 1000   // 5 minutes
export interface AgentActivity { active: boolean; relativeTime: string }
export function agentActivity(lastActivityAt: string, now: Date): AgentActivity
```
Takes `now` as a parameter so it is unit-testable without fake timers, and clamps future
timestamps/clock skew to `{active: true, "just now"}`. Reuse it anywhere a relative time or an
active/idle decision is needed.

**`ui/src/components/AgentRow.tsx`** — one session row: live `StatusDot` (pulses only while
active), project link, mono `#ticket_id` + title linking to `/p/:project_id/t/:ticket_id`, mono
branch, mono relative time.

**Depends on ticket 008's `/api/agents` shape.** Two things to know:
- `session_url` is `""` in v1 (routine sessions aren't persisted), so the "Session" link is
  rendered **only when it is non-empty** — it activates automatically if the backend ever
  starts populating it. Both states are covered by tests.
- `started_at`/`last_activity_at` are the branch tip's commit time, so "last activity" tracks
  the agent's last commit, not keystrokes. That is the honest v1 signal; if it ever needs to be
  finer, the backend must persist real session activity.

The pulsing live dot stays the app's only standing animation (DESIGN.md §2.4);
`prefers-reduced-motion: reduce` disables it through the token layer.
