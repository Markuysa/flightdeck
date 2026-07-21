# ADR-001: Derive ticket status, never store it

Date: 2026-07-21
Status: accepted

## Context
FlightDeck is a dashboard over a queue that already has a source of truth: the ticket
files and git state in each project. A dashboard traditionally keeps its own database and
syncs. Sync drifts, and a control surface showing stale status is worse than no surface.

## Decision
FlightDeck stores no ticket status. On every board read it recomputes status from the
project's `docs/tickets/*.md` plus git and PR state, using the exact rules the agents use
(`derive` package mirrors `docs/tickets/README.md`). The only stored data is the registry:
which projects exist and their secrets.

## Consequences
- The board cannot disagree with the agents — it computes the same function they do.
- Reads cost git/API work; mitigated by short-TTL caching and SSE invalidation, never by
  persisting status.
- A `status` column for tickets would reintroduce exactly the drift this avoids. It is
  forbidden, and CLAUDE.md says so.
