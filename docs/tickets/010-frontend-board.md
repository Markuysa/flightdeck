---
id: 10
title: Frontend — Board (kanban) screen
role: frontend
depends: [2]
status: done
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

The Board screen (`ui/src/pages/Board.tsx`) is real. **Ticket 011 (ticket detail) reuses the
dependency trail** built here — do not rebuild it.

**`ui/src/components/DependencyTrail.tsx` (reuse verbatim):**
```ts
export interface DependencyTrailProps {
  depends: number[]                        // upstream ticket ids, in file order
  statuses: Record<number, DerivedStatus>  // upstream id -> derived status
  className?: string
}
```
Renders `null` when `depends` is empty; otherwise one `StatusDot` per id coloured by
`statuses[id]` (falls back to `blocked` if an id is missing — defensive only). This is
DESIGN.md §3's dependency trail: a blocked ticket shows at a glance which upstream is red.

**How to build the `statuses` map — ticket 011:** the ticket-detail endpoint returns
`TicketDetail.depends_detail: BoardTicket[]`, so:
```ts
const statuses = Object.fromEntries(detail.depends_detail.map(t => [t.id, t.status]))
<DependencyTrail depends={detail.depends} statuses={statuses} />
```
(The Board screen instead flattens the six board buckets into that same id→status map with a
`useMemo`, since it has the whole board — no extra request either way.)

**Also reusable:** `components/TicketCard.tsx` (mono `#id`, title, role chip, trail; a real
`<Link>` to `/p/:id/t/:tid`) and `components/Column.tsx` (300px, `StatusChip` header). Board
data comes from `getBoard(id)` → `Board = Record<DerivedStatus, BoardTicket[]>` (all six keys
present, empty = `[]`). Columns/order come from `STATUS_ORDER`.

**Live updates:** `useFlightDeckEvents(() => refetch())` — refetch on any of the three events
(only `dispatch.started` fires today; the board-diff publisher is a later follow-up per ticket
008's handoff). Same testing gotchas as ticket 009 apply (`resetAllMocks`, local
`afterEach(cleanup)`, `Promise.resolve().then(load)`). Below the `board:` (1280px) breakpoint
the board scrolls horizontally, not the page.
