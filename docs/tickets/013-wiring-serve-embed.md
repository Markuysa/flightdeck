---
id: 13
title: Wiring — composition root, serve command, UI embed
role: backend
depends: [3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
status: done
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

## Handoff

FlightDeck is now one binary. `internal/app` is the composition root (ADR-004) — the only
package that imports concrete implementations; every feature package still depends solely on
`internal/core`. Tickets 014 (qa e2e) and 015 (readme) build on what follows.

**Configuration (env):**

| Var | Default | Notes |
|---|---|---|
| `FLIGHTDECK_TOKEN` | *(none — required)* | Startup fails fast with a clear error if empty |
| `FLIGHTDECK_ADDR` | `:8080` | API and UI share this one port |
| `FLIGHTDECK_DB` | `flightdeck.db` | SQLite registry file; gitignored |

**Build and run (verbatim — ticket 015's readme needs exactly this):**
```sh
# 1. build the UI and copy it into the embed directory
cd ui && npm ci && npm run build && cd ..
cp -r ui/dist/. internal/webui/dist/

# 2. build the single binary
go build -o bin/flightdeck ./cmd/flightdeck

# 3. run it
FLIGHTDECK_TOKEN=<random-secret> ./bin/flightdeck serve
# dev loop instead: FLIGHTDECK_TOKEN=<secret> go run ./cmd/flightdeck serve
```
`flightdeck version` prints the version (`dev` unless set via `-ldflags "-X main.version=…"`).

**Register the first project** (the API is cookie-authenticated; trade the bearer once):
```sh
curl -X POST -H "Authorization: Bearer $FLIGHTDECK_TOKEN" \
  http://localhost:8080/api/session -c cookies.txt

curl -b cookies.txt -X POST -H "Content-Type: application/json" \
  -d '{"name":"My Project","repo_path":"/absolute/path/to/a/checkout"}' \
  http://localhost:8080/api/projects

curl -b cookies.txt http://localhost:8080/api/projects/my-project/board
```
`repo_path` is a **local checkout** whose `docs/tickets/*.md` FlightDeck reads. Add
`"github": {"owner":"…","repo":"…"}` to enable PR/CI state; without it the board still renders
from git alone. The project id is a slug of the name.

**The UI embed — read this before touching it.** `internal/webui` embeds
`internal/webui/dist`, **not** `ui/dist`. `//go:embed` fails at *compile time* when its pattern
matches no files, and `ui/dist` is gitignored and absent on a clean checkout — embedding it
would turn every CI run red. A tracked placeholder `internal/webui/dist/index.html` keeps the
pattern non-empty; real assets copied over it are gitignored
(`internal/webui/dist/*` with `!internal/webui/dist/index.html`). **If you skip step 1 above the
binary still runs — it just serves the placeholder instead of the app.** Unknown non-`/api`
paths fall back to the shell, so `/p/:id`, `/p/:id/t/:tid` and `/agents` survive a hard refresh.

**For ticket 014 (e2e):** boot the real thing the way `internal/app/smoke_test.go` does —
`git.NewFixtureRepo` for a project, a temp `FLIGHTDECK_DB`, `httptest`/`:0` listener, then
`POST /api/session` → `POST /api/projects` → `GET /api/projects/{id}/board`. It runs fully
offline (a project with no github remote never calls GitHub). For browser-level e2e, serve the
built UI (step 1) so the embedded app, not the placeholder, is under test.
