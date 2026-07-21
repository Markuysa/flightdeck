# ADR-004: Composition root in internal/app

Date: 2026-07-21
Status: accepted

## Context
Nine-ish feature packages built by parallel tickets. If they import each other, the
tickets stop being independent and merges collide.

## Decision
Feature packages depend only on `internal/core`. Only `internal/app` imports concrete
implementations and wires them. A test asserts `internal/core` imports nothing internal.

## Consequences
- Tickets are parallelizable — the reason this project can be built by the agent team.
- One place (app) knows the whole graph; everywhere else is a leaf.
