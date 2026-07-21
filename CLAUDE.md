# FlightDeck

A dashboard for driving a team of coding agents. It renders the file-based ticket queues
of one or more projects, shows which agents are working right now, and lets a human
dispatch work and approve merges — the CEO console for the vibe-coding setup.

## What it reads, and why that shapes everything

FlightDeck owns **no queue of its own**. Its source of truth is other repositories:
`docs/tickets/*.md` plus git branch and PR state, exactly as `next-ticket` and `/board`
derive it. This is deliberate — a dashboard with its own database would drift from what
the agents actually see. FlightDeck computes, never stores, ticket status.

## Stack

- **Backend:** Go 1.22+, single binary, serves the API and embeds the built UI (`embed.FS`)
- **UI:** React + Vite + TypeScript + Tailwind, built to static assets, embedded
- **Data sources, read side:** local git repositories (go-git or shelling to `git`), and
  the GitHub REST API via a token for PR/CI state
- **Data sources, write side:** the Claude Code Routines `/fire` API to dispatch tickets;
  `gh`/`glab`/git to merge on human approval
- **Store:** none for domain data. A small SQLite file only for local UI config
  (which projects are registered, routine tokens) — never for ticket status

## Commands

Backend (repo root):
- dev: `go run ./cmd/flightdeck serve`
- test: `go test ./...`
- lint: `golangci-lint run`
- build: `go build -o bin/flightdeck ./cmd/flightdeck`

UI (`ui/`):
- dev: `npm run dev`
- test: `npm test`
- lint: `npm run lint`
- build: `npm run build`

## Rules

- Non-trivial changes go through a plan first: propose, then write code.
- Tests are mandatory for business logic. The status-derivation engine is the core of the
  product — it is tested against fixture repositories (a temp git repo with known tickets
  and branches), never against a live remote.
- **Never store ticket status.** If you find yourself writing a `status` column for
  tickets, stop — status is derived from git on read. Persisting it is the one design
  mistake this product exists to avoid.
- Design tokens come only from `docs/DESIGN.md`. Hardcoded colours or spacing are a defect.
- Routine tokens and GitHub tokens are secrets: environment or the local config store,
  never committed, never logged, never sent to the browser.
- The dashboard dispatches and merges only on explicit human action. No auto-anything on
  the server — autopilot lives in the routines, not here.

## References (read on demand, do not hold in context)
- Spec: docs/PRD.md
- Architecture: docs/ARCHITECTURE.md
- Design system: docs/DESIGN.md
- Decisions (ADR): docs/decisions/
- The queue model this reads: the vibe-coding-template's docs/tickets/README.md

## Compaction policy
When compacting, preserve: the full list of changed files, test commands, and decisions
with their reasoning. Condense research findings aggressively.
