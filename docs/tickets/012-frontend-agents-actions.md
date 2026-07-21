---
id: 12
title: Frontend — Agents live view
role: frontend
depends: [2]
status: todo
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
