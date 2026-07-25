---
id: 17
title: Autopilot toggle in the UI
role: frontend
depends: [9, 13]
status: done
---
The Fleet card shows autopilot On/Off read-only; the PUT endpoint and api client exist but
nothing flips it (US-5). Add the control.

## Acceptance criteria
- [ ] The project card's autopilot state becomes an interactive toggle that calls
      `setAutopilot(id, on)` and reflects the result; it optimistically or on-success updates
      the shown state, and surfaces an API error without corrupting the displayed state.
- [ ] The toggle is a real, keyboard-accessible control (button with `aria-pressed`, or a
      switch role), disabled while the request is in flight.
- [ ] Tokens/design: only DESIGN.md tokens; on = accent, off = quiet, per the design system.
- [ ] Behaviour test against a mocked client: toggling calls setAutopilot with the flipped
      value and updates; an error is surfaced and the state is not left wrong. lint + build +
      test pass.

## Handoff

The Fleet project card's autopilot state is now an interactive `role="switch"` toggle
(`ProjectCard.tsx`) calling `setAutopilot(project.id, next)`. It seeds from `project.autopilot`
and updates **only from the server's response** — never optimistically — so a rejected request
leaves the shown state correct; the error surfaces via a `role="alert"`. The control is disabled
while the request is in flight, has an `aria-label`, and uses `--accent` for its on-state (a
control, per §1, not a status colour) and quiet tokens for off. Closes US-5's UI gap; the
PUT `/api/projects/{id}/autopilot` endpoint (ticket 007/008) is unchanged.
