---
id: 2
title: UI scaffold — tokens, shell, status primitives
role: designer
depends: []
status: todo
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
Write what tokens and primitives now exist and what the frontend screen tickets should
import — they build entirely on this.
