---
name: spec
description: Turns a rough idea into docs/PRD.md with real acceptance criteria, by interviewing one question at a time.
argument-hint: [the idea, one line]
disable-model-invocation: true
allowed-tools: Read, Write, Edit, Glob, Grep
---

The idea: $ARGUMENTS

Produce `docs/PRD.md`. Use `docs/PRD-template.md` as the shape if it exists.

## Standing rule: one question at a time

This applies for the whole conversation, not just your first reply.

Ask **one** question, wait for the answer, then ask the next. Never send a numbered list
of questions — a wall of questions gets a wall of shallow answers, and the point of this
command is to surface the things the person has not thought about yet.

Prefer questions that change what gets built. Skip questions whose answer you could
reasonably assume; state the assumption instead and let them correct it. When you have
enough for a section, say so and move on rather than interrogating past the point of value.

Stop when you can write acceptance criteria that someone else could verify without asking
you anything. That is the finish line, not "when the template has no blanks left".

## What to find out

- Who has the problem, and what they do today instead
- What changes for them when this ships — observable, not aspirational
- The smallest version that is still worth shipping
- What is explicitly **out** of scope (this section prevents more rework than any other)
- Constraints that are already fixed: stack, deadlines, existing systems, compliance
- How you would know it failed

## Acceptance criteria are the deliverable

Everything else in the PRD is context for these. Each criterion must be:

- **Checkable** — a person or a test can say met/not met without interpretation
- **About behavior**, not implementation ("returns 401 for an expired token", not "uses middleware")
- **Bounded** — covers a stated case, including the edge case that motivated it

`validator` will verify against exactly this list, and `dev` will refuse a ticket without
it. Vague criteria here become expensive misunderstandings three agents downstream.

Before writing the file, show the acceptance criteria and confirm them. Then write
`docs/PRD.md` and say what is still unresolved.
