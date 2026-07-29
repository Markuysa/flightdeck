package plan

import (
	"errors"
	"strings"
	"testing"
)

func tk(number int, deps ...int) Ticket {
	return Ticket{Number: number, Title: "T", Role: "backend", Depends: deps}
}

func planOf(tickets ...Ticket) Plan { return Plan{Tickets: tickets} }

func TestValidateAcceptsAcyclicPlan(t *testing.T) {
	t.Parallel()
	p := planOf(tk(1), tk(2, 1), tk(3, 1), tk(4, 2, 3))
	if err := Validate(p, DefaultMaxTickets); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// TestValidateRejectsCycles is the check that matters most. `blocked` is derived
// from `depends` on every board read, so a cycle does not error at read time —
// it produces tickets that can never become ready, and a queue that silently
// never starts.
func TestValidateRejectsCycles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		plan Plan
	}{
		{"two-ticket cycle", planOf(tk(1, 2), tk(2, 1))},
		{"three-ticket cycle", planOf(tk(1, 3), tk(2, 1), tk(3, 2))},
		{"cycle behind an acyclic prefix", planOf(tk(1), tk(2, 1), tk(3, 4), tk(4, 3))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tc.plan, DefaultMaxTickets)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("Validate = %v, want ErrInvalidPlan", err)
			}
			if !strings.Contains(err.Error(), "cycle") {
				t.Errorf("error = %q, want it to name the cycle", err)
			}
		})
	}
}

func TestValidateRejectsSelfDependency(t *testing.T) {
	t.Parallel()
	err := Validate(planOf(tk(1, 1)), DefaultMaxTickets)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("Validate = %v, want ErrInvalidPlan", err)
	}
	if !strings.Contains(err.Error(), "itself") {
		t.Errorf("error = %q, want it to name the self-dependency", err)
	}
}

// TestValidateRejectsDanglingDependency: a dependency on a ticket that does not
// exist blocks the dependent forever, with no error anywhere downstream.
func TestValidateRejectsDanglingDependency(t *testing.T) {
	t.Parallel()
	err := Validate(planOf(tk(1), tk(2, 99)), DefaultMaxTickets)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("Validate = %v, want ErrInvalidPlan", err)
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error = %q, want it to name the missing dependency", err)
	}
}

func TestValidateRejectsMalformedTickets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		plan Plan
	}{
		{"empty plan", planOf()},
		{"zero number", planOf(Ticket{Number: 0, Title: "T", Role: "backend"})},
		{"negative number", planOf(Ticket{Number: -1, Title: "T", Role: "backend"})},
		{"duplicate numbers", planOf(tk(1), tk(1))},
		{"empty title", planOf(Ticket{Number: 1, Title: "  ", Role: "backend"})},
		{"unknown role", planOf(Ticket{Number: 1, Title: "T", Role: "wizard"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(tc.plan, DefaultMaxTickets); !errors.Is(err, ErrInvalidPlan) {
				t.Errorf("Validate = %v, want ErrInvalidPlan", err)
			}
		})
	}
}

// TestValidateEnforcesTicketCap: approving a plan is one click that commits an
// unbounded amount of agent work, so the ceiling has to actually hold.
func TestValidateEnforcesTicketCap(t *testing.T) {
	t.Parallel()
	var tickets []Ticket
	for i := 1; i <= 6; i++ {
		tickets = append(tickets, tk(i))
	}
	if err := Validate(planOf(tickets...), 5); !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("Validate with 6 tickets against a cap of 5 = %v, want ErrInvalidPlan", err)
	}
	if err := Validate(planOf(tickets[:5]...), 5); err != nil {
		t.Errorf("Validate at exactly the cap: %v", err)
	}
}

// TestCycleReportIsDeterministic: an error message that changes between
// identical runs is a bad error message.
func TestCycleReportIsDeterministic(t *testing.T) {
	t.Parallel()
	p := planOf(tk(1, 4), tk(2, 1), tk(3, 2), tk(4, 3))
	first := Validate(p, DefaultMaxTickets).Error()
	for range 20 {
		if got := Validate(p, DefaultMaxTickets).Error(); got != first {
			t.Fatalf("cycle report changed between runs:\n %q\n %q", first, got)
		}
	}
}
