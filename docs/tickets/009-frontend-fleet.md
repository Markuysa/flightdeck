---
id: 9
title: Frontend — Fleet screen (all projects)
role: frontend
depends: [2]
status: todo
---
The landing view (US-1): every registered project as a card with per-status counts,
autopilot state, and a live-agent indicator. Register/remove a project.

## Likely files
- `ui/src/pages/Fleet.tsx`, `ui/src/components/{ProjectCard,RegisterProjectDialog}.tsx`

## Acceptance criteria
- [ ] Fleet renders projects from `GET /api/projects`, each card showing name, StatusChip
      counts (all six statuses), autopilot on/off, and a live dot when an agent is active.
- [ ] Register dialog posts `{name, repo_path, github?}`; validation errors shown inline
      from the API 422.
- [ ] Empty state with a hint when no projects are registered (PRD §7 first-run matters).
- [ ] Only DESIGN.md tokens; behaviour tested (renders counts, opens dialog, handles error)
      against a mocked client. lint + build + test pass.

## Handoff
Note any shared hook (e.g. useProjects) the other screens reuse.
