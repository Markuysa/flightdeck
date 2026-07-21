---
id: 10
title: Frontend — Board (kanban) screen
role: frontend
depends: [2]
status: todo
---
One project as a kanban (US-2): columns per derived status, cards with id/title/role/deps.

## Likely files
- `ui/src/pages/Board.tsx`, `ui/src/components/{Column,TicketCard}.tsx`

## Acceptance criteria
- [ ] Columns in pipeline order (ready → in progress → in review → needs attention →
      blocked → done) from `GET /api/projects/{id}/board`; each column headed by a
      StatusChip count.
- [ ] Card shows mono id, title, role chip, and a dependency dot-trail; clicking opens the
      ticket route.
- [ ] Live updates: subscribes to SSE `board.changed` and refetches.
- [ ] Horizontal scroll below 1280px per §2.4; tokens only; behaviour tested against a
      mocked board. lint + build + test pass.

## Handoff
Note the dependency-trail component if the ticket screen reuses it.
