---
id: 11
title: Frontend — Ticket detail screen
role: frontend
depends: [2]
status: todo
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
Note the action-button component if Agents reuses dispatch.
