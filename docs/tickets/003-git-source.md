---
id: 3
title: Git source — branch and merged-ness state
role: backend
depends: [1]
status: done
---
Implement `core.GitState` and `core.TicketReader` over a local repo, reading tickets from
`docs/tickets/*.md` and answering branch/merge questions the derive engine needs.

## Likely files
- `internal/source/git/{git,tickets,fixture}.go`

## Acceptance criteria
- [ ] `Tickets` parses every `docs/tickets/[0-9]*.md`: id, title, role, depends, body, and
      the `## Handoff` section when present. Malformed frontmatter is reported, not panicked.
- [ ] `GitState` answers `Branches`, `IsMergedToMain`, and `FileOnBranch` correctly.
- [ ] A fixture helper builds a temp git repo with given tickets/branches/merges, used by
      tests here and reusable by the derive tests. No network, no shared global state.
- [ ] Round-trip tests over the fixture: a merged branch reads merged; a file on a branch
      reads its branch version, not main's.
- [ ] lint + `go test ./...` pass.

## Handoff

Package `internal/source/git` implements the two `core` read interfaces over a local
checkout by shelling to the `git` binary (no go-git dependency; `go.mod` stays stdlib-only).
Tickets 004 (github), 005 (derive), 008 (api) and 014 (qa) build on this.

**`Repo` — `internal/source/git/git.go`.** `type Repo struct { Path, Main string }`;
`NewRepo(path string) *Repo` defaults `Main` to `"main"`. Implements both `core.GitState`
and `core.TicketReader` (compile-time asserted). Methods:
- `Branches(ctx) ([]string, error)` — local heads **and** remote-tracking branches by short
  name, deduplicated, excluding the default branch and symbolic refs like `origin/HEAD`.
  Remote-tracking `claude/*` branches are included on purpose, so a branch pushed by an
  agent but never checked out locally is still visible to derive.
- `IsMergedToMain(ctx, branch) (bool, error)` — resolves a short branch name across
  `refs/heads/<b>` then each remote's `refs/remotes/<remote>/<b>`; unknown branch → error.
- `FileOnBranch(ctx, branch, path) (string, error)` — the file's contents **on that branch**,
  not main (`git show <ref>:<path>`).

**Ticket reading — `internal/source/git/tickets.go`.** Two entry points:
- `Tickets(ctx) ([]core.Ticket, error)` — the plain `core.TicketReader`.
- `TicketsWithStatus(ctx) ([]TicketMeta, error)` — **this is what the derive engine wants.**
  `type TicketMeta struct { core.Ticket; RawStatus string }` carries the literal frontmatter
  `status:` value (`todo` / `done` / `needs-attention`) **beside** `core.Ticket`, never on it
  (ADR-001). Both read `docs/tickets/[0-9]*.md` from `Repo.Path` on disk (README.md is
  excluded by the glob). Malformed frontmatter returns an error naming the file — the whole
  read fails, it never panics or silently skips.
- Note the split: these disk readers read the working checkout. To get a ticket file's
  `status` **as it stands on a specific branch** (what derive needs to tell in_progress from
  in_review), pair `FileOnBranch(ctx, branch, "docs/tickets/NNN-*.md")` with the same
  frontmatter parsing — the derive ticket composes `GitState` + this parsing; it does not
  need a new git method.

**Fixture helper — `internal/source/git/fixture.go` (NON-test file, so other packages'
tests import it). Tickets 005 and 014 reuse this verbatim; the signature is stable:**
```go
type FixtureTicket struct { ID int; Title, Role string; Depends []int; Status, Body string }
type FixtureBranch struct { Name string; Tickets []FixtureTicket; Merged bool }
func NewFixtureRepo(t *testing.T, tickets []FixtureTicket, branches []FixtureBranch) *Repo
```
Builds a temp git repo (`t.TempDir()`, local `user.email`/`user.name`, no network, no shared
state, `t.Parallel()`-safe) with `tickets` committed to `docs/tickets` on `main`, then each
branch created in order — `FixtureBranch.Tickets` overrides those ticket files on the branch
(typically to flip `Status`), and `Merged: true` merges it back to main with `--no-ff`.
Returns a ready `*Repo`. Fixture ticket files are named `NNN-fixture.md` by id.

**Tooling note (not this ticket's code):** running bare `golangci-lint run` from the repo
root also lints `ui/node_modules/**` (a vendored Go shim inside an npm dep), which is
gitignored and absent from CI checkouts. Lint the module explicitly with `./internal/...`
locally; CI is unaffected.
