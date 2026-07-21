# ADR-002: Single binary with embedded UI

Date: 2026-07-21
Status: accepted

## Context
FlightDeck is self-hosted by one person managing their own agent team. Deployment friction
is the enemy of "I'll just run it".

## Decision
One Go binary serves the API and the built React UI from `embed.FS`. `flightdeck serve`
and it is up. Same shape as the projects it manages.

## Consequences
- No separate frontend host, no CORS, no reverse proxy for the common case.
- The UI build is a step in the Go build; CI builds both.
- Rejected: separate SPA + API deployment. Too much operational surface for one user.
