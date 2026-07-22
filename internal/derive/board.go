package derive

import "github.com/Markuysa/flightdeck/internal/core"

// StatusOrder is the canonical column order the frontend renders, matching
// ui/src/lib/status.ts's STATUS_ORDER exactly (ticket 002's handoff):
// ready, in_progress, in_review, needs_attention, blocked, done.
var StatusOrder = []core.DerivedStatus{
	core.StatusReady,
	core.StatusInProgress,
	core.StatusInReview,
	core.StatusNeedsAttention,
	core.StatusBlocked,
	core.StatusDone,
}

// Board groups Derive's output by DerivedStatus. This is what ticket 008's
// GET /api/projects/{id}/board serializes: the map's key type
// (core.DerivedStatus, a string) and value type ([]core.BoardTicket) marshal
// to exactly the shape ticket 002's frontend already types as
// `Board = Record<DerivedStatus, BoardTicket[]>` in ui/src/lib/types.ts.
//
// Every status in StatusOrder is present as a key, holding an empty (never
// nil) slice when no ticket has that status — the API always serves all six
// buckets rather than omitting empty ones, and an empty slice encodes as
// JSON `[]`, not `null`, which is what the frontend's Record type expects to
// index without a nil check.
//
// Ticket order within each bucket follows the input slice's order. Derive
// already returns tickets sorted by id, so passing its output straight
// through preserves id order per bucket; Board itself does no additional
// sorting or shuffling.
//
// The map itself carries no group ordering (Go maps do not preserve
// insertion order and JSON object key order is not meaningful to callers
// here) — callers that need the display order iterate StatusOrder
// separately, exactly as the frontend's STATUS_ORDER does.
func Board(tickets []core.BoardTicket) map[core.DerivedStatus][]core.BoardTicket {
	board := make(map[core.DerivedStatus][]core.BoardTicket, len(StatusOrder))
	for _, s := range StatusOrder {
		board[s] = []core.BoardTicket{}
	}
	for _, t := range tickets {
		board[t.Status] = append(board[t.Status], t)
	}
	return board
}
