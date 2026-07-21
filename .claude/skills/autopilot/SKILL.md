---
name: autopilot
description: Starts, stops, or reports on unattended execution of the whole ticket queue. Refuses to start unless CI is actually gating merges.
argument-hint: "on | off | status"
disable-model-invocation: true
allowed-tools: Read, Glob, Edit, Bash(gh pr *), Bash(gh repo view *), Bash(gh api repos/*), Bash(git *), Skill
---

Requested: $ARGUMENTS (default `status`)

The state lives in `.claude/autopilot.json`, committed so cloud sessions read it from the
clone and every flip is auditable. `next-ticket` refuses to work while `enabled` is false.

## `status`

Report, in this order: whether autopilot is on, how many tickets are in progress against
`maxInFlight`, how many are ready, how many are blocked and on what, and whether any sit
at needs-attention — those need a human and will never clear on their own. Derive all of
it from `docs/tickets/` plus branches, the way `/board` does.

## `on`

### Preflight — all four, no exceptions

Autopilot without a real merge gate is not automation, it is merging whatever was written.
Check every item and **refuse if any fails**, naming the failure and the fix:

1. **A CI workflow exists** — something under `.github/workflows/`.
2. **Branch protection requires status checks.** Read it:
   `gh api repos/{owner}/{repo}/branches/{default}/protection`
   `required_status_checks.contexts` must be non-empty. If protection is absent or the list
   is empty, `gh pr merge --auto` merges on the spot and autopilot means nothing. This is
   the check that matters most — do not wave it through because a workflow file exists.
3. **Auto-merge is enabled on the repository** — `gh api repos/{owner}/{repo} -q .allow_auto_merge`
   must be `true`. (`gh repo view --json autoMergeAllowed` does not exist; the field is only
   exposed by the REST API.)
4. **There is at least one ready ticket** in `docs/tickets/`: `status: todo` in main, all
   `depends` done, no `claude/NNN-*` branch. Use the same derivation `/board` uses.

Autopilot's auto-merge requires a GitHub remote: checks 1–3 are what make `--auto` defer
to CI instead of merging blind, and only GitHub enforces them server-side here. On GitLab
or a local-only repository, refuse `on` and point at the manual loop instead — "take the
next ticket", human merges. The queue itself works everywhere; unattended merging is the
part that needs a server-side gate.

Report which checks passed even when you refuse. "Preflight failed" without a reason wastes
the next ten minutes.

### Start

Set `enabled` to true in `.claude/autopilot.json`, commit and push it — the cloud reads the
committed copy, so an uncommitted edit changes nothing.

Then invoke `next-ticket` with autopilot mode to start the first ticket.

Explain what happens next in two sentences: the pull request auto-merges once CI is green,
the merge fires the routine, and the routine runs `unblock` then `next-ticket` again. The
chain is self-sustaining from here — nobody has to dispatch anything.

Finish by naming the stop command: `/autopilot off`.

## `off`

Set `enabled` to false, commit, push. Report anything currently `in-progress`: that run
finishes on its own — this stops the *next* ticket from starting, it does not kill work
already in flight. Say that explicitly rather than implying everything halted.

To stop a run already going, the human interrupts that session; to stop the chain from
restarting on the next merge, pause the routine at claude.ai/code/routines.

## Rules

- Never edit `.claude/autopilot.json` outside this command.
- Never turn autopilot on because the queue looks ready. Preflight decides, every time.
- `maxInFlight` above 1 is the human's call, never yours. With `strict` branch protection
  each merge invalidates every other open branch, so a wider queue thrashes on rebases.
