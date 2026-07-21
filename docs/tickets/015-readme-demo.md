---
id: 15
title: README, quickstart, and demo mode
role: dev
depends: [13]
status: todo
---
Make it runnable by a newcomer: README, and a `--demo` mode that registers a seeded fixture
project so the UI is explorable with no real repo.

## Likely files
- `README.md`, `internal/demo/seed.go`, wire `--demo` into serve

## Acceptance criteria
- [ ] README: what it is, the derive-never-store principle, how to build, run, register a
      project, set the routine/GitHub tokens, and the security note (tokens never in the
      browser).
- [ ] `flightdeck serve --demo` starts with a seeded fixture project showing tickets in
      several statuses, no real git remote required.
- [ ] A screenshot or asciicast of the Fleet and Board views in the README.
- [ ] lint + tests pass.
