---
id: 13
title: Wiring — composition root, serve command, UI embed
role: backend
depends: [3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
status: todo
---
`internal/app` composes every implementation; `cmd/flightdeck serve` runs it; `webui`
embeds `ui/dist`. This is where the parallel tickets become one binary.

## Likely files
- `internal/app/app.go`, `internal/webui/embed.go`, `cmd/flightdeck/main.go`

## Acceptance criteria
- [ ] `internal/app` builds the git+github sources, derive engine, registry, dispatcher and
      API from config; only this package imports concrete implementations (ADR-004).
- [ ] `flightdeck serve` starts the server, serving the API and the embedded UI from one
      port; `flightdeck version` prints the version.
- [ ] `go build ./...` produces the single binary; the embedded UI loads with no external
      request. Graceful shutdown.
- [ ] A smoke test boots the server against a fixture project and gets a derived board over
      HTTP. lint + tests pass.

## Handoff
Note how to run it and register the first project — the readme ticket needs this.
