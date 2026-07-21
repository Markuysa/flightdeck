---
id: 3
title: Git source — branch and merged-ness state
role: backend
depends: [1]
status: todo
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
Note the fixture-repo helper's signature — the derive and qa tickets reuse it.
