package derive

import (
	"math/rand"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

// want describes the fields of the derived core.BoardTicket a test cares
// about; PR is compared by value (nil means "no PR annotation expected").
type want struct {
	status core.DerivedStatus
	branch string
	pr     *core.PRState
}

func TestDerive_RuleTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		tickets    []core.Ticket
		mainStatus map[int]string
		branches   map[int]BranchState
		prs        map[string]core.PRState
		want       map[int]want
	}{
		{
			name:       "done on main, no branch",
			tickets:    []core.Ticket{{ID: 1, Title: "Foundation"}},
			mainStatus: map[int]string{1: "done"},
			want:       map[int]want{1: {status: core.StatusDone}},
		},
		{
			name:       "done on main wins over a lingering branch that says needs-attention",
			tickets:    []core.Ticket{{ID: 1, Title: "Foundation"}},
			mainStatus: map[int]string{1: "done"},
			branches:   map[int]BranchState{1: {Name: "claude/001-x", FileStatus: "needs-attention"}},
			want:       map[int]want{1: {status: core.StatusDone, branch: "claude/001-x"}},
		},
		{
			name:       "needs_attention: branch says needs-attention, main todo",
			tickets:    []core.Ticket{{ID: 5, Title: "Attention"}},
			mainStatus: map[int]string{5: "todo"},
			branches:   map[int]BranchState{5: {Name: "claude/005-attention", FileStatus: "needs-attention"}},
			want:       map[int]want{5: {status: core.StatusNeedsAttention, branch: "claude/005-attention"}},
		},
		{
			name:       "needs_attention wins over an open PR for the branch (precedence)",
			tickets:    []core.Ticket{{ID: 5, Title: "Attention"}},
			mainStatus: map[int]string{5: "todo"},
			branches:   map[int]BranchState{5: {Name: "claude/005-attention", FileStatus: "needs-attention"}},
			prs:        map[string]core.PRState{"claude/005-attention": {Number: 9, URL: "https://x/9", CI: "green"}},
			want:       map[int]want{5: {status: core.StatusNeedsAttention, branch: "claude/005-attention", pr: nil}},
		},
		{
			name:       "in_review: branch file done, main still todo, no PR",
			tickets:    []core.Ticket{{ID: 7, Title: "Review"}},
			mainStatus: map[int]string{7: "todo"},
			branches:   map[int]BranchState{7: {Name: "claude/007-review", FileStatus: "done"}},
			want:       map[int]want{7: {status: core.StatusInReview, branch: "claude/007-review", pr: nil}},
		},
		{
			name:       "in_review annotated with its open PR and CI state",
			tickets:    []core.Ticket{{ID: 7, Title: "Review"}},
			mainStatus: map[int]string{7: "todo"},
			branches:   map[int]BranchState{7: {Name: "claude/007-review", FileStatus: "done"}},
			prs:        map[string]core.PRState{"claude/007-review": {Number: 42, URL: "https://x/42", CI: "pending"}},
			want: map[int]want{7: {
				status: core.StatusInReview,
				branch: "claude/007-review",
				pr:     &core.PRState{Number: 42, URL: "https://x/42", CI: "pending"},
			}},
		},
		{
			name:       "in_progress: branch file still todo",
			tickets:    []core.Ticket{{ID: 3, Title: "Progress"}},
			mainStatus: map[int]string{3: "todo"},
			branches:   map[int]BranchState{3: {Name: "claude/003-progress", FileStatus: "todo"}},
			want:       map[int]want{3: {status: core.StatusInProgress, branch: "claude/003-progress"}},
		},
		{
			name:       "blocked: no branch, a dependency is not done on main",
			tickets:    []core.Ticket{{ID: 10, Title: "Blocked", Depends: []int{2}}},
			mainStatus: map[int]string{10: "todo", 2: "todo"},
			want:       map[int]want{10: {status: core.StatusBlocked}},
		},
		{
			name:       "ready: no branch, todo, every dependency done on main",
			tickets:    []core.Ticket{{ID: 11, Title: "Ready", Depends: []int{2}}},
			mainStatus: map[int]string{11: "todo", 2: "done"},
			want:       map[int]want{11: {status: core.StatusReady}},
		},
		{
			name:       "ready: no branch, todo, no dependencies at all",
			tickets:    []core.Ticket{{ID: 12, Title: "Ready no deps"}},
			mainStatus: map[int]string{12: "todo"},
			want:       map[int]want{12: {status: core.StatusReady}},
		},
		{
			// Acceptance criterion: a lying stored status must never override
			// the computed rule. The file literally claims "ready" but an
			// unmet dependency must still derive blocked.
			name:       "lying status: stored 'ready' with an unmet dependency still derives blocked",
			tickets:    []core.Ticket{{ID: 20, Title: "Liar", Depends: []int{2}}},
			mainStatus: map[int]string{20: "ready", 2: "todo"},
			want:       map[int]want{20: {status: core.StatusBlocked}},
		},
		{
			// A stored "in_progress" with no branch is never trusted either;
			// it is computed like any other non-done/needs-attention value.
			name:       "lying status: stored 'in_progress' with no branch and no unmet deps computes ready",
			tickets:    []core.Ticket{{ID: 21, Title: "Liar 2"}},
			mainStatus: map[int]string{21: "in_progress"},
			want:       map[int]want{21: {status: core.StatusReady}},
		},
		{
			name:       "lying status: stored 'in_progress' with no branch but an unmet dependency computes blocked",
			tickets:    []core.Ticket{{ID: 22, Title: "Liar 3", Depends: []int{2}}},
			mainStatus: map[int]string{22: "in_progress", 2: "todo"},
			want:       map[int]want{22: {status: core.StatusBlocked}},
		},
		{
			name:       "missing mainStatus entry defaults to todo: ready when no deps",
			tickets:    []core.Ticket{{ID: 30, Title: "No entry"}},
			mainStatus: map[int]string{},
			want:       map[int]want{30: {status: core.StatusReady}},
		},
		{
			name:       "missing mainStatus entry for a dependency counts as not done: blocked",
			tickets:    []core.Ticket{{ID: 31, Title: "Depends on unknown", Depends: []int{99}}},
			mainStatus: map[int]string{31: "todo"},
			want:       map[int]want{31: {status: core.StatusBlocked}},
		},
		{
			name:       "nil maps behave like empty maps (total function, no panics)",
			tickets:    []core.Ticket{{ID: 40, Title: "All nil"}},
			mainStatus: nil,
			branches:   nil,
			prs:        nil,
			want:       map[int]want{40: {status: core.StatusReady}},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Derive(tc.tickets, tc.mainStatus, tc.branches, tc.prs)
			if len(got) != len(tc.tickets) {
				t.Fatalf("Derive returned %d tickets, want %d (must be total)", len(got), len(tc.tickets))
			}
			for _, bt := range got {
				w, ok := tc.want[bt.ID]
				if !ok {
					t.Fatalf("unexpected ticket id %d in output", bt.ID)
				}
				assertBoardTicket(t, bt, w)
			}
		})
	}
}

func assertBoardTicket(t *testing.T, got core.BoardTicket, w want) {
	t.Helper()
	if got.Status != w.status {
		t.Errorf("ticket %d: Status = %q, want %q", got.ID, got.Status, w.status)
	}
	if got.Branch != w.branch {
		t.Errorf("ticket %d: Branch = %q, want %q", got.ID, got.Branch, w.branch)
	}
	switch {
	case got.PR == nil && w.pr == nil:
		// both absent, fine
	case got.PR == nil || w.pr == nil:
		t.Errorf("ticket %d: PR = %v, want %v", got.ID, got.PR, w.pr)
	case *got.PR != *w.pr:
		t.Errorf("ticket %d: PR = %+v, want %+v", got.ID, *got.PR, *w.pr)
	}
}

// TestDerive_Deterministic asserts the output is always sorted by ticket id,
// regardless of the input slice's order.
func TestDerive_Deterministic(t *testing.T) {
	t.Parallel()

	tickets := []core.Ticket{
		{ID: 5}, {ID: 1}, {ID: 3}, {ID: 4}, {ID: 2},
	}
	mainStatus := map[int]string{}

	rng := rand.New(rand.NewSource(42))
	for attempt := 0; attempt < 5; attempt++ {
		shuffled := append([]core.Ticket(nil), tickets...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		got := Derive(shuffled, mainStatus, nil, nil)
		if len(got) != 5 {
			t.Fatalf("attempt %d: got %d tickets, want 5", attempt, len(got))
		}
		for i, want := range []int{1, 2, 3, 4, 5} {
			if got[i].ID != want {
				t.Errorf("attempt %d: got[%d].ID = %d, want %d (must sort by id)", attempt, i, got[i].ID, want)
			}
		}
	}
}

// TestDerive_TotalOneBoardTicketPerInput asserts every input ticket produces
// exactly one output BoardTicket, including tickets absent from every map.
func TestDerive_TotalOneBoardTicketPerInput(t *testing.T) {
	t.Parallel()

	tickets := []core.Ticket{{ID: 1}, {ID: 2}, {ID: 3}}
	got := Derive(tickets, nil, nil, nil)
	if len(got) != len(tickets) {
		t.Fatalf("Derive returned %d BoardTickets for %d input tickets, want a 1:1 mapping", len(got), len(tickets))
	}
}
