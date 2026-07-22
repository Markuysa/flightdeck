---
id: 9
title: Frontend — Fleet screen (all projects)
role: frontend
depends: [2]
status: done
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

The Fleet screen (`ui/src/pages/Fleet.tsx`) is real. Screens 010–012 should reuse the shared
pieces below rather than re-fetching or re-styling.

**Shared hook — `ui/src/hooks/useProjects.ts`:**
```ts
const { projects, loading, error, refresh, register, remove } = useProjects()
// projects: ProjectSummary[]  (Project + counts/autopilot/hasLiveAgent)
// register(body: CreateProjectRequest): Promise<void>  — rejects on API failure WITHOUT
//   refreshing, so the caller renders the server's message inline
// remove(id: string): Promise<void>  — refreshes on success
```
Use it anywhere the registered-project list is needed (e.g. a project switcher). It loads once
on mount; call `refresh()` after a mutation or an SSE `board.changed`.

**Components to reuse:**
- `components/ProjectCard.tsx` — a whole project card: name + repo path linked to `/p/:id`,
  one `StatusChip` per `STATUS_ORDER` entry, autopilot on/off, a pulsing live `StatusDot`
  (`status="in_progress" live`) when `hasLiveAgent`, and remove.
- `components/RegisterProjectDialog.tsx` — `{ onClose, onRegister }`. **It has no `open` prop
  by design** — mount it conditionally (`{open && <RegisterProjectDialog … />}`) so every open
  is a fresh blank form. Escape/backdrop close it, focus starts on the first field and returns
  to the trigger, and API errors render inline (`role="alert"`) without closing. Copy this
  pattern for any other dialog (012's actions).

**API error handling — `ui/src/lib/api.ts` changed:** `request()` now parses the backend's real
`{"error": "..."}` body (matching `internal/api`'s `errorBody`) into `ApiError.message` for any
4xx/5xx. So `catch (e) { (e as ApiError).message }` gives you the server's message directly —
use it for 012's dispatch (409 not-ready) and approve errors instead of inventing text.

**Testing conventions established (follow these — they cost time to find):**
- Mock the client with `vi.mock('../lib/api')`; use `vi.resetAllMocks()` in `beforeEach`
  (`clearAllMocks` leaves queued `mockResolvedValueOnce` values behind).
- `vite.config.ts` has no `test.globals`, so RTL auto-cleanup is not registered — add
  `afterEach(cleanup)` **locally in each test file** or earlier tests' DOM pollutes later queries.
- `eslint-plugin-react-hooks`'s `set-state-in-effect` flags an effect that transitively reaches
  a `setState`, even across an `await`. Wrap a mount-time loader as
  `Promise.resolve().then(() => refresh())` to satisfy it.

Styling: only token classes (`bg-surface`, `text-mut`, `border-soft`, `rounded-card`,
`font-display`, `font-mono`, …). A raw hex anywhere in `ui/src/` fails `npm run lint`.
