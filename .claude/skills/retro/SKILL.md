---
name: retro
description: Finds what you had to explain more than once during this work and proposes concrete diffs to CLAUDE.md, skills, or agent roles so the next project starts smarter.
argument-hint: [optional: what to focus on]
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Edit, Bash(git log *), Bash(git diff *)
---

Focus: $ARGUMENTS

The template is meant to be a compounding asset. This command is how it compounds: turn
what was learned into configuration, so it is not re-learned next time.

## What to look for

Go back over this session and find:

1. **Repeated corrections.** Anything the human had to say more than once — a convention,
   a preference, a constraint, a "no, not like that". Every repetition is a missing line
   of configuration.
2. **Wrong assumptions.** Where an agent guessed about the codebase and guessed wrong.
   That is usually a gap in `CLAUDE.md` or in `docs/QUICK_REF.md`.
3. **Friction.** Steps that were manual and did not need to be. Permission prompts that
   fired repeatedly for something obviously safe.
4. **Role failures.** Cases where an agent did something outside its remit, or lacked a
   tool it genuinely needed.

## Where each fix belongs

| Finding | Goes to |
|---|---|
| A project fact, always true, needed every session | `CLAUDE.md` — but keep it under 200 lines |
| A procedure with steps, needed occasionally | a new skill in `.claude/skills/` |
| How one role should behave | that role in `.claude/agents/` |
| A safe command prompting every time | `permissions.allow` in `.claude/settings.json` |
| A decision and its reasoning | a new ADR in `docs/decisions/` |
| A recurring gotcha | `docs/QUICK_REF.md`, if it stays under 50 lines |

Prefer skills over `CLAUDE.md` when there is any doubt: `CLAUDE.md` costs context in every
session, a skill costs nothing until invoked.

## Rules

- **Propose concrete diffs**, not advice. "Be clearer about testing" is useless; the exact
  lines to add to `dev.md` are not.
- **Cite the evidence.** For each proposal, name what happened that justifies it. A
  proposal you cannot ground in something that actually occurred is speculation, and it
  will cost context forever while helping nobody.
- **Cut as well as add.** If something in `CLAUDE.md` or a role went unused or actively
  misled, propose removing it. Configuration that only grows becomes noise.
- Be honest about your reach: you see this session. Patterns from other sessions are not
  visible to you, so do not present a single occurrence as a trend.

Show the proposed diffs and let the human choose. Apply only what they approve, then
remind them these changes belong committed back to the template repository, not just to
the current project.
