---
id: 6
title: Registry — projects and secrets store
role: backend
depends: [1]
status: done
---
Persist the one thing FlightDeck legitimately owns: which projects are registered and
their secrets. SQLite-backed, secrets never exposed (ADR-005). No ticket status here ever.

## Likely files
- `internal/registry/{registry,sqlite,migrations}.go`

## Acceptance criteria
- [ ] CRUD for `Project` (add/list/get/remove) backed by SQLite via `modernc.org/sqlite`
      (no cgo); migrations embedded, idempotent on open.
- [ ] Per-project secrets (routine token, github token) stored separately and returned
      only to server-side callers; a `Project` DTO for the API carries no secret.
- [ ] The DB file path is configurable and gitignored; a store round-trip test runs against
      a temp file, not `:memory:`.
- [ ] No table stores ticket status (ADR-001); a test asserts the schema has no such column.
- [ ] lint + tests pass.

## Handoff

Package `internal/registry` persists registered projects + their secrets in SQLite
(`modernc.org/sqlite`, pure Go). Ticket 008 (api) opens one `Store` at startup and uses it
for the projects endpoints and to source per-project tokens for the git/github/dispatch
sources.

**Open/lifecycle:**
```go
func Open(path string) (*Store, error) // path is caller-supplied + gitignored; migrations run idempotently on open
func (s *Store) Close() error
```

**Project CRUD — returns `core.Project`, the secret-free API DTO** (`{ID, Name, RepoPath,
Remote, Owner, Repo}`, no token fields — safe to serialize straight to the browser):
```go
func (s *Store) Add(ctx, p core.Project) error          // p.ID unique; dup ID -> UNIQUE constraint error
func (s *Store) List(ctx) ([]core.Project, error)
func (s *Store) Get(ctx, id string) (core.Project, error)   // missing -> core.ErrProjectNotFound (errors.Is)
func (s *Store) Remove(ctx, id string) error                // missing -> core.ErrProjectNotFound
```
`POST /api/projects` (register) → `Add`; `GET /api/projects` → `List` (then run each through
derive for status counts); `DELETE /api/projects/{id}` → `Remove`.

**Secrets — SERVER-SIDE ONLY, never in an API response:**
```go
type Secrets struct { RoutineToken string; GitHubToken string }
func (s *Store) Secrets(ctx, projectID string) (Secrets, error)    // missing project -> ErrProjectNotFound; zero value if none set
func (s *Store) SetSecrets(ctx, projectID string, sec Secrets) error
```
`Secrets` lives in a separate `project_secrets` table, is never embedded in `core.Project`,
and its `String()`/`GoString()` are redacted (`%v` prints `registry.Secrets{redacted}`). The
api layer reads `Secrets` to construct `github.New(owner, repo, sec.GitHubToken)` and the
routine dispatcher (ticket 007) — but must NEVER put a token in any handler's JSON. Registering
a project with its tokens (US-7) means `Add` then `SetSecrets`; the request DTO accepts tokens
inbound, but no response DTO ever returns them.

**Dependency / toolchain note:** this ticket added the module's first external dependency,
`modernc.org/sqlite`. Its module graph requires **go 1.25.0**, so `go.mod`'s directive moved
from `1.22` to `1.25.0` (unavoidable without pinning the whole modernc/x tree). CI installs Go
via `go-version-file: go.mod`, so it follows automatically; `go.sum` is committed. Downstream
tickets need no action, but don't be surprised the floor is now 1.25.
