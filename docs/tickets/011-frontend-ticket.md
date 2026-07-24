---
id: 11
title: Frontend — Ticket detail screen
role: frontend
depends: [2]
status: done
---
Ticket detail (US-3): body, acceptance criteria, dependency trail, upstream handoffs, and
PR/CI when in review — plus the action buttons.

## Likely files
- `ui/src/pages/Ticket.tsx`, `ui/src/components/{DependencyTrail,HandoffList,PRBadge}.tsx`

## Acceptance criteria
- [ ] Renders body + criteria from `GET .../tickets/{tid}`, the dependency trail as status
      dots, and each dependency's handoff text.
- [ ] When in review, shows PR number/link and CI state coloured per the CI mapping.
- [ ] Action area: a Dispatch button enabled only when the ticket is `ready`, and an
      Approve-merge button only when `in_review` (US-6). Actions call the API and surface
      the result (session URL / merge outcome); they never merge client-side.
- [ ] Tokens only; behaviour tested for the enable/disable logic against a mocked client.
      lint + build + test pass.

## Handoff

The Ticket detail screen (`ui/src/pages/Ticket.tsx`) is real. **Ticket 012 (Agents) reuses the
action component** — do not rebuild dispatch/approve.

**`ui/src/components/TicketActions.tsx` (reuse for 012's dispatch):**
```ts
export interface TicketActionsProps {
  projectId: string
  ticketId: number
  status: DerivedStatus
  className?: string
}
```
Self-contained — owns its own request + result/error state. Renders a **Dispatch** button,
`disabled` (real HTML attribute) unless `status === 'ready'`, that calls
`dispatchTicket(projectId, {ticket_id})` and surfaces the returned `session_url`; on a 409 it
shows `ApiError.message`. An **Approve-merge** button renders only when `status === 'in_review'`
and calls `approveTicket`. Neither merges/dispatches client-side. Mount it anywhere you know a
`{projectId, ticketId, status}` triple.

**Other reusable components:**
- `components/PRBadge.tsx` — `{ pr: PRState }`. PR number linked to `pr.url` (mono) + CI state
  coloured per ticket 004's mapping (`pending→--st-progress`, `green→--st-done`,
  `red→--st-attention`, `unknown→--text-dim`) via an inline `style` CSS var (same trick as
  `StatusDot` for a dynamically-chosen colour — not a hex, so the lint guard passes).
- `components/HandoffList.tsx` — `{ deps: BoardTicket[] }`, renders each dependency's `handoff`
  text; `null` when none have one yet.
- `lib/markdown.tsx` — `renderTicketBody(body: string): ReactNode`, a small line-based renderer
  (headings, `- [ ]`/`- [x]` checklists via lucide icons, bullets, paragraphs). **No markdown
  dependency was added** — reuse this rather than pulling one in.

**Testing gotcha (adds to ticket 009's three):** a full `vi.mock('../lib/api')` automock replaces
the `ApiError` class, breaking `instanceof ApiError` / `.message`. When a test needs real
`ApiError` behaviour (404/409 handling), use a **partial mock**:
`vi.mock('../lib/api', async (importOriginal) => ({ ...(await importOriginal()), getTicket: vi.fn() }))`.

Tokens only; ids/PR numbers/branches mono; raw hex outside tokens.css fails `npm run lint`.
