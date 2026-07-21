---
name: ship
description: Pre-merge gate for the current branch — runs validator and reviewer, then summarizes the diff for a human decision. Never merges.
argument-hint: [optional: base branch, defaults to the repo default]
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Bash(git *), Bash(gh *), Agent
---

Base branch: $ARGUMENTS

## Current state

- Branch: !`git branch --show-current`
- Default branch: !`git symbolic-ref --short refs/remotes/origin/HEAD`
- Uncommitted: !`git status --porcelain`
- Recent commits: !`git log --oneline -20`
- Unstaged diff size: !`git diff --stat`

If those came back empty or as errors, this is not a git repository or has no remote —
say so and stop; the rest of this gate assumes both.

## Gate

Run these in order. **A failure at any step stops the gate** — report and stop rather than
continuing to the next step.

1. **Uncommitted work.** If the working tree is dirty, say so and stop. Shipping a branch
   whose state you cannot name is how work gets lost.
2. **Validator.** Delegate to the `validator` subagent with the ticket's acceptance
   criteria. Take its verdict as given — do not re-interpret RED as "mostly fine".
3. **Reviewer.** Only if the validator returned GREEN. Delegate to the `reviewer` subagent
   against the base branch.
4. **Summary for the human.** Everything above condensed into something readable in a
   minute (see below).

## Summary format

- **What this branch does** — one or two sentences, in terms of user-visible behavior
- **Acceptance criteria** — each one, met or not
- **Validator** — GREEN/RED with the actual failing output if RED
- **Reviewer** — blocking findings first; say plainly if there are none
- **Risk** — what could break in production that the tests do not cover
- **Recommendation** — mergeable or not, and why

## Hard rule

**You do not merge, push, or create a release.** Those are the human's call, and the
permission rules in `.claude/settings.json` are set up to ask before any of them.
Prepare the decision; do not make it.

If the gate passes cleanly, say so without hedging. If it does not, lead with what failed.
