# ADR-005: Secrets in a local config store, never in the browser

Date: 2026-07-21
Status: accepted

## Context
FlightDeck holds routine dispatch tokens and a GitHub token. It has a browser UI. Tokens
that reach the browser or the committed config are leaked tokens.

## Decision
Secrets live in a local SQLite config store (file-permission protected), read server-side
only. The browser never receives a token; it calls FlightDeck's API, and FlightDeck calls
out with the secret. The API auth is a single `FLIGHTDECK_TOKEN` traded for a session
cookie. Nothing secret is logged.

## Consequences
- The dashboard is a confused-deputy on purpose: the browser asks, the server acts with
  the secret. Standard, and the only safe shape here.
- Registry file must not be committed; .gitignore covers it.
