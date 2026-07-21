---
name: board
description: Renders the whole ticket queue with derived statuses — the board view of docs/tickets/. Works from git alone, on any hosting or none.
argument-hint: "[optional: status filter, e.g. ready]"
allowed-tools: Read, Glob, Bash(git *), Bash(gh pr list *)
---

Filter: $ARGUMENTS

Render the queue in `docs/tickets/`. Statuses are **derived, never trusted from memory**
— compute each one exactly as `docs/tickets/README.md` defines:

1. Read every `docs/tickets/[0-9]*.md` on the current main. Parse frontmatter:
   `id`, `title`, `role`, `depends`, `status`.
2. List ticket branches: `git branch -a --list '*claude/*'` (local **and** remote).
3. For each ticket, derive:
   - `done` — status is `done` in main
   - `needs attention` — its branch exists and the file **in that branch**
     (`git show <branch>:docs/tickets/<file>`) says `needs-attention`
   - `in review` — branch exists, file in branch says `done`, main still says `todo`
   - `in progress` — branch exists, file in branch still says `todo`
   - `blocked` — no branch, and some id in `depends` is not `done` in main
   - `ready` — no branch, todo, all depends done
4. If `gh` is available and the repo has a GitHub remote, annotate in-review rows with
   their PR number and CI state (`gh pr list --head <branch>`). Skip silently otherwise —
   the board must render identically on GitLab or with no remote at all.

## Output

One table, ordered by id, grouped by status in pipeline order:
ready → in progress → in review → needs attention → blocked → done.

```
READY        003  Hacker News collector        role:dev  deps:1✓
IN REVIEW    007  LLM providers                role:dev  PR #31 CI:green
BLOCKED      019  Wiring                       waits on: 10, 12, 15
```

End with one summary line: `N done · N in flight · N ready · N blocked` — and, if
anything sits at needs-attention, name it explicitly: those never clear on their own.

Read-only: this skill never edits a ticket, never creates a branch, never merges.
