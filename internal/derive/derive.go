// Package derive is the status engine: it turns a project's tickets plus
// precomputed git and PR state into the derived board the API serves and
// the frontend renders. It mirrors the rules docs/tickets/README.md and
// docs/ARCHITECTURE.md's "Status derivation" section define for the agents
// themselves, so the dashboard can never disagree with what an agent sees
// (ADR-001: derive status on every read, never store it).
//
// Derive is a pure, total function: no I/O, no clock. Every input — main
// branch statuses, per-ticket branch state, open PRs — is precomputed by
// the caller (internal/source/git and internal/source/github), which is
// what makes the whole rule table unit-testable against hand-built maps
// instead of a live repo or network call.
package derive

import (
	"sort"

	"github.com/Markuysa/flightdeck/internal/core"
)

// BranchState is what the caller resolved for a ticket that has a
// claude/NNN-* branch: the branch's name and the raw `status:` literal the
// ticket file carries on that branch — not on main. Callers build this by
// pairing core.GitState.FileOnBranch with the same frontmatter parsing
// internal/source/git uses for TicketsWithStatus (see that package's
// TicketMeta).
type BranchState struct {
	Name       string // the branch name, e.g. "claude/007-dispatch-fire-autopilot-approve"
	FileStatus string // the raw status on that branch: "todo" | "done" | "needs-attention"
}

// Derive computes every ticket's DerivedStatus and returns one
// core.BoardTicket per ticket in tickets, sorted by Ticket.ID ascending
// (deterministic regardless of input order — the core of the "pure and
// total" requirement).
//
// Inputs, all precomputed by the caller — Derive performs no I/O itself:
//
//   - tickets: every ticket to derive, in any order.
//   - mainStatus: ticket id -> the raw `status:` frontmatter literal as it
//     reads on main ("todo" | "done" | "needs-attention"). A ticket with no
//     entry is treated as "todo": a ticket the caller hasn't recorded a
//     status for yet must never be silently treated as done, nor make a
//     dependent ticket unblockable forever.
//   - branches: ticket id -> BranchState, present ONLY when a claude/NNN-*
//     branch exists for that ticket. A missing entry means no branch.
//   - prs: branch name -> core.PRState, keyed exactly as core.PRReader's
//     OpenPRs returns. May be nil (e.g. the GitHub source is unavailable
//     and the caller downgraded per ARCHITECTURE.md's failure policy) — a
//     missing entry simply means no PR annotation, never an error.
//
// Rule precedence — exactly ARCHITECTURE.md's "Status derivation" section,
// and the ADR-001 constraint that a ticket's stored status is trusted only
// for the two literals "done" and "needs-attention"; in_progress, in_review,
// blocked and ready are always computed, never read off the file as-is:
//
//  1. done            — mainStatus == "done". Wins over everything else,
//     including a lingering branch: once main says done, the ticket is done.
//  2. needs_attention — a branch exists and its FileStatus == "needs-attention".
//  3. in_review       — a branch exists, its FileStatus == "done", and main
//     is not (rule 1 already returned for main == "done", so reaching here
//     main is anything else, i.e. still "todo" from derive's perspective).
//  4. in_progress     — a branch exists and its FileStatus is anything else
//     (normally "todo").
//  5. blocked         — no branch, and some Depends id is not "done" on
//     main (mainStatus only — a dependency's own in-progress branch never
//     counts, only its merge to main does).
//  6. ready           — no branch, and every Depends id is "done" on main.
//
// BoardTicket.Branch is set whenever branches has an entry for the ticket,
// independent of the derived status (e.g. a done ticket whose branch the
// caller hasn't pruned from its map yet still reports it). BoardTicket.PR
// is set only when the derived status is in_review and prs has an entry
// for that branch — matching core.BoardTicket's own doc comment ("PR, when
// in review"): a PR opened for a needs_attention or already-done ticket is
// not surfaced through this field.
func Derive(
	tickets []core.Ticket,
	mainStatus map[int]string,
	branches map[int]BranchState,
	prs map[string]core.PRState,
) []core.BoardTicket {
	board := make([]core.BoardTicket, 0, len(tickets))
	for _, t := range tickets {
		board = append(board, deriveOne(t, mainStatus, branches, prs))
	}
	sort.Slice(board, func(i, j int) bool { return board[i].ID < board[j].ID })
	return board
}

func deriveOne(
	t core.Ticket,
	mainStatus map[int]string,
	branches map[int]BranchState,
	prs map[string]core.PRState,
) core.BoardTicket {
	bt := core.BoardTicket{Ticket: t}

	bs, hasBranch := branches[t.ID]
	if hasBranch {
		bt.Branch = bs.Name
	}

	if rawStatus(mainStatus, t.ID) == "done" {
		bt.Status = core.StatusDone
		return bt
	}

	if hasBranch {
		switch bs.FileStatus {
		case "needs-attention":
			bt.Status = core.StatusNeedsAttention
		case "done":
			bt.Status = core.StatusInReview
			if pr, ok := prs[bs.Name]; ok {
				prCopy := pr
				bt.PR = &prCopy
			}
		default:
			bt.Status = core.StatusInProgress
		}
		return bt
	}

	if allDependsDone(t.Depends, mainStatus) {
		bt.Status = core.StatusReady
	} else {
		bt.Status = core.StatusBlocked
	}
	return bt
}

// rawStatus returns the raw status literal on main for id, defaulting to
// "todo" when mainStatus has no entry for it.
func rawStatus(mainStatus map[int]string, id int) string {
	if s, ok := mainStatus[id]; ok {
		return s
	}
	return "todo"
}

// allDependsDone reports whether every id in depends reads "done" on main.
// It always consults mainStatus, never a dependency's own branch state — a
// dependency only satisfies "depends" once it is merged to main.
func allDependsDone(depends []int, mainStatus map[int]string) bool {
	for _, dep := range depends {
		if rawStatus(mainStatus, dep) != "done" {
			return false
		}
	}
	return true
}
