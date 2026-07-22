---
id: 2
title: UI scaffold — tokens, shell, status primitives
role: designer
depends: []
status: done
---
Stand up the React/Vite/TS/Tailwind app, encode `docs/DESIGN.md` §2 tokens once, and build
the shell plus the StatusDot/StatusChip primitives every screen depends on. No backend
dependency — codes against the frozen API contract in ARCHITECTURE.md.

## Likely files
- `ui/package.json`, `ui/vite.config.ts`, `ui/tailwind.config.ts`, `ui/tsconfig.json`
- `ui/src/styles/tokens.css`, `ui/src/main.tsx`, `ui/src/App.tsx`
- `ui/src/lib/api.ts` (typed client for the whole contract), `ui/src/lib/sse.ts`
- `ui/src/components/{Shell,StatusDot,StatusChip,Chip,Button,EmptyState}.tsx`

## Acceptance criteria
- [ ] Vite + React + TS + Tailwind; `npm run build`, `npm run lint`, `npm test` pass.
- [ ] Every DESIGN.md §2 token (incl. the six status colours §2.2) defined once as a CSS
      variable, surfaced via tailwind.config; a hex literal anywhere in `ui/src/` fails the
      lint (guard script).
- [ ] Space Grotesk, Inter, JetBrains Mono self-hosted via @fontsource; no runtime CDN request.
- [ ] `StatusDot` and `StatusChip` render each of the six statuses with the exact §2.2
      colour; snapshot-tested for all six.
- [ ] Shell: collapsible sidebar (Fleet / Agents), dark-only, routes for `/`, `/p/:id`,
      `/p/:id/t/:tid`, `/agents`.
- [ ] Typed API client covers every endpoint in the ARCHITECTURE.md contract + an SSE hook.
- [ ] Vite dev server proxies `/api` to the Go process. Focus states and
      prefers-reduced-motion per §2.4.

## Handoff

The UI app exists at `ui/` (Vite + React 18 + TS + Tailwind v3, dark-only). Screen tickets
009–012 fill in the route components; everything below already exists — import it, never
rebuild it, and never write a raw colour.

**Commands** (run in `ui/`): `npm run dev`, `npm run lint`, `npm test`, `npm run build`.
CI runs `npm ci` → lint → test → build on Node 22, so keep `ui/package-lock.json` committed.

**Tokens — `ui/src/styles/tokens.css`.** Every DESIGN.md §2 value, defined exactly once:
§2.1 colours (`--bg`, `--surface`, `--surface-2`, `--border`, `--border-soft`, `--text`,
`--text-mut`, `--text-dim`, `--accent`, `--accent-soft`), the six §2.2 status colours
(`--st-ready|progress|review|attention|blocked|done`), §2.3 font families
(`--font-display|ui|mono`) and §2.4 radii (`--radius-card|chip|nested`).
Surfaced through `ui/tailwind.config.ts` as theme values, so use Tailwind classes:
`bg-surface`, `text-mut`, `border-soft`, `rounded-card`, `rounded-chip`, `rounded-nested`,
`font-display`, `font-mono`, etc.
**A hex literal anywhere in `ui/src/` outside tokens.css fails `npm run lint`**
(`ui/scripts/check-tokens.mjs`). That is intended — add a token instead.

**Status primitives — the one element repeated everywhere:**
- `components/StatusDot.tsx` — `{ status: DerivedStatus; live?: boolean; className?: string }`.
  `live` turns on the 2s pulse (the live-agent indicator, §3); it is off by default.
- `components/StatusChip.tsx` — `{ status: DerivedStatus; count: number; className?: string }`,
  for per-status counts on project cards.
- `lib/status.ts` — `STATUS_ORDER` (the canonical column/display order) and `STATUS_META`
  (`label` + `colorVar` per status). Use these for labels and ordering rather than
  hardcoding; board columns should iterate `STATUS_ORDER`.
Both primitives are snapshot-tested across all six statuses (13 tests). If you change their
markup, update snapshots deliberately.

**Other primitives:** `components/Button.tsx` (`variant?: 'primary' | 'ghost'`, plus native
button props), `components/EmptyState.tsx` (`{ icon: ComponentType<{className?}>; title;
description?; action? }`). Icons come from `lucide-react` — SVG only, never emoji.

**Shell & routing — `components/Shell.tsx`, `App.tsx`.** Collapsible sidebar (Fleet /
Agents) that collapses to a top bar at 860px. Routes already wired:
`/` → `pages/Fleet.tsx`, `/p/:id` → `pages/Board.tsx`, `/p/:id/t/:tid` → `pages/Ticket.tsx`,
`/agents` → `pages/Agents.tsx`. **Those four page files are placeholders — tickets 009–012
replace their bodies.** Keep the file paths and route shape.

**Typed API client — `lib/api.ts`** covers the whole frozen contract; call these rather than
`fetch`: `listProjects`, `createProject`, `deleteProject`, `getBoard`, `getTicket`,
`dispatchTicket`, `getAutopilot`, `setAutopilot`, `approveTicket`, `listAgents`,
`createSession`. Auth is a session cookie obtained via `createSession` (`POST /api/session`);
never put a token in localStorage or a URL.

**Types — `lib/types.ts`** mirror `internal/core`: `DerivedStatus` (the six exact backend
strings `ready|in_progress|in_review|blocked|needs_attention|done`), `Project`,
`ProjectSummary`, `Ticket`, `BoardTicket`, `PRState`, `TicketDetail`,
`Board` (= `Record<DerivedStatus, BoardTicket[]>`), `CreateProjectRequest`,
`DispatchRequest`/`DispatchResponse`, `AutopilotState`, `AgentSession`.
**Backend (ticket 008) must serve these exact shapes** — the board endpoint returns tickets
grouped by status as `Board`.

**Live updates — `lib/sse.ts`:** `useFlightDeckEvents` subscribes to `GET /api/events` and
cleans up on unmount; `EVENT` holds the event-kind constants. Use it to invalidate/refetch
rather than polling.

**Conventions:** fonts are self-hosted via `@fontsource` (no CDN request — keep it that
way). Focus is `:focus-visible` only, 2px `--accent` outline at 2px offset. Transitions are
150ms hover-only, and everything is disabled under `prefers-reduced-motion: reduce`.
Vite dev proxies `/api` → `http://localhost:8080` (the Go process).
