# FlightDeck — Design System

> Source of truth for every UI decision. The `designer` role owns this file; `frontend`
> consumes it and never inlines a raw value.

## 1. Concept

**Metaphor: mission control.** FlightDeck is where one person watches a team of agents
work. It is a monitoring surface that lives on a second screen — calm at rest, legible at
a glance, with only the things that need a human lit up. Status is the entire visual
language: everything else is quiet so status reads instantly.

Anti-patterns (forbidden):
- AI-gradient purple→pink and glow effects — cliché, and noise on a monitoring surface
- Emoji as status icons — SVG only (lucide-react)
- Light theme as default — this runs on a spare monitor
- More than one accent competing for attention — status colours are the accent

## 2. Tokens

### 2.1 Colour

| Token | HEX | Role |
|---|---|---|
| `--bg` | `#0A0C10` | App background (near-black ink) |
| `--surface` | `#111419` | Cards, panels, columns |
| `--surface-2` | `#161A21` | Hover, raised elements |
| `--border` | `#222831` | Active borders, dividers |
| `--border-soft` | `#1A1F27` | Quiet card borders |
| `--text` | `#E6E9EF` | Primary text |
| `--text-mut` | `#8A93A3` | Secondary text |
| `--text-dim` | `#586274` | Metadata, labels, disabled |
| `--accent` | `#4C8DFF` | Interactive: links, active nav, primary buttons, focus |
| `--accent-soft` | `rgba(76,141,255,.12)` | Active chip/row background |

### 2.2 Status colour — the core semantic

One colour per derived status. These are the only meaningful colours in the app; never
reuse them decoratively.

| Status | Token | HEX |
|---|---|---|
| ready | `--st-ready` | `#4C8DFF` (accent blue — actionable) |
| in progress | `--st-progress` | `#FFB224` (amber — working) |
| in review | `--st-review` | `#A879FF` (violet — awaiting human) |
| needs attention | `--st-attention` | `#FF6B4A` (hot — a human must look) |
| blocked | `--st-blocked` | `#586274` (dim — waiting, not urgent) |
| done | `--st-done` | `#3DD68C` (green — merged) |

### 2.3 Typography

| Role | Font | Use |
|---|---|---|
| Display | Space Grotesk 600 | page titles, project names |
| UI/Body | Inter 400/500/600 | all interface text |
| Data | JetBrains Mono 400/500 | ticket ids, counts, branch names, timestamps |

Rule: every number and identifier is mono. Section labels are mono uppercase 10.5px,
letter-spacing 1px, `--text-dim`.

### 2.4 Geometry & motion

- Radius: cards `10px`, chips `999px`, nested `6px`
- Kanban column width 300px; project cards in a responsive grid, min 260px
- Breakpoints: 1280px (board goes horizontal-scroll), 860px (sidebar collapses to top bar)
- Transitions 150ms on hover only; the sole standing animation is a 2s pulse on the
  live-agent dot. `prefers-reduced-motion: reduce` disables all of it.
- Focus: 2px `--accent` outline, 2px offset, on `:focus-visible` only.

## 3. Signature elements

- **StatusDot / StatusChip** — a filled dot (kanban card) or a chip with count (project
  card), coloured by §2.2. This is the one element repeated everywhere; get it right once.
- **Live indicator** — a pulsing `--st-progress` dot next to an agent that is currently
  working. The only motion in the app.
- **Dependency trail** — on a ticket detail, its `depends` shown as a small chain of
  status dots, so a blocked ticket shows at a glance which upstream is red.

## 4. Screens (frontend tickets build these)

1. **Fleet** — all projects as cards, each with name, per-status counts, autopilot state,
   a live-agent indicator. The landing view.
2. **Board** — one project as a kanban: columns per status, cards with id/title/role/deps.
3. **Ticket** — body, acceptance criteria, dependency trail, upstream handoffs, PR/CI when
   in review, and the actions (dispatch if ready, approve-merge if in review).
4. **Agents** — live sessions: who is working, on which ticket, last activity.

## 5. Implementation

- React + Vite + Tailwind; tokens above become `tailwind.config` theme referencing CSS
  variables defined once in `src/styles/tokens.css`.
- Fonts self-hosted (`@fontsource`), no runtime CDN request.
- Icons `lucide-react`. Charts: none in v1 (this is a control surface, not analytics).
- Dark theme only (`color-scheme: dark`).
