package derive

import (
	"reflect"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

func TestStatusOrder_MatchesFrontendOrder(t *testing.T) {
	t.Parallel()
	want := []core.DerivedStatus{
		core.StatusReady,
		core.StatusInProgress,
		core.StatusInReview,
		core.StatusNeedsAttention,
		core.StatusBlocked,
		core.StatusDone,
	}
	if !reflect.DeepEqual(StatusOrder, want) {
		t.Fatalf("StatusOrder = %v, want %v (must match ui/src/lib/status.ts's STATUS_ORDER)", StatusOrder, want)
	}
}

func TestBoard_GroupsByStatus(t *testing.T) {
	t.Parallel()

	tickets := []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
		{Ticket: core.Ticket{ID: 2}, Status: core.StatusDone},
		{Ticket: core.Ticket{ID: 3}, Status: core.StatusInProgress},
	}

	board := Board(tickets)

	if len(board[core.StatusReady]) != 1 || board[core.StatusReady][0].ID != 1 {
		t.Errorf("board[ready] = %v, want a single ticket id 1", board[core.StatusReady])
	}
	if len(board[core.StatusDone]) != 1 || board[core.StatusDone][0].ID != 2 {
		t.Errorf("board[done] = %v, want a single ticket id 2", board[core.StatusDone])
	}
	if len(board[core.StatusInProgress]) != 1 || board[core.StatusInProgress][0].ID != 3 {
		t.Errorf("board[in_progress] = %v, want a single ticket id 3", board[core.StatusInProgress])
	}
}

func TestBoard_EveryStatusKeyPresentEvenWhenEmpty(t *testing.T) {
	t.Parallel()

	board := Board(nil)
	for _, s := range StatusOrder {
		got, ok := board[s]
		if !ok {
			t.Fatalf("board is missing key %q, want every StatusOrder status present", s)
		}
		if got == nil {
			t.Errorf("board[%q] = nil, want an empty (non-nil) slice so JSON encodes [] not null", s)
		}
		if len(got) != 0 {
			t.Errorf("board[%q] = %v, want empty", s, got)
		}
	}
	if len(board) != len(StatusOrder) {
		t.Errorf("board has %d keys, want exactly %d (one per StatusOrder entry)", len(board), len(StatusOrder))
	}
}

func TestBoard_PreservesIDOrderWithinBucket(t *testing.T) {
	t.Parallel()

	// Derive already sorts by id; Board must not reshuffle a bucket once
	// tickets land in it.
	tickets := []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusBlocked},
		{Ticket: core.Ticket{ID: 4}, Status: core.StatusBlocked},
		{Ticket: core.Ticket{ID: 7}, Status: core.StatusBlocked},
	}

	board := Board(tickets)

	wantIDs := []int{1, 4, 7}
	got := board[core.StatusBlocked]
	if len(got) != len(wantIDs) {
		t.Fatalf("board[blocked] has %d tickets, want %d", len(got), len(wantIDs))
	}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Errorf("board[blocked][%d].ID = %d, want %d", i, got[i].ID, want)
		}
	}
}

// TestBoard_EndToEndWithDerive checks Board grouping on Derive's own output,
// so the two files agree on the contract the API composes them through.
func TestBoard_EndToEndWithDerive(t *testing.T) {
	t.Parallel()

	tickets := []core.Ticket{
		{ID: 1, Title: "Done"},
		{ID: 2, Title: "Ready"},
		{ID: 3, Title: "Blocked", Depends: []int{2}},
		{ID: 4, Title: "In progress"},
		{ID: 5, Title: "In review"},
		{ID: 6, Title: "Needs attention"},
	}
	mainStatus := map[int]string{1: "done", 2: "todo", 3: "todo", 4: "todo", 5: "todo", 6: "todo"}
	branches := map[int]BranchState{
		4: {Name: "claude/004-x", FileStatus: "todo"},
		5: {Name: "claude/005-x", FileStatus: "done"},
		6: {Name: "claude/006-x", FileStatus: "needs-attention"},
	}

	board := Board(Derive(tickets, mainStatus, branches, nil))

	checks := map[core.DerivedStatus]int{
		core.StatusDone:           1,
		core.StatusReady:          2,
		core.StatusBlocked:        3,
		core.StatusInProgress:     4,
		core.StatusInReview:       5,
		core.StatusNeedsAttention: 6,
	}
	for status, wantID := range checks {
		bucket := board[status]
		if len(bucket) != 1 || bucket[0].ID != wantID {
			t.Errorf("board[%q] = %v, want a single ticket id %d", status, bucket, wantID)
		}
	}
}
