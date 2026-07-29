package plan

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// DefaultMaxTickets bounds one plan. Approving a plan is one click that commits
// an unbounded amount of agent work, so the ceiling is deliberate rather than
// generous.
const DefaultMaxTickets = 20

// ErrInvalidPlan wraps every validation failure. It is distinct from
// ErrPlanFailed so a caller can tell "the model produced a graph we refuse to
// write" from "we could not reach the model at all".
var ErrInvalidPlan = errors.New("plan: invalid")

// Validate rejects a plan the derive engine could not survive.
//
// The three graph rules exist because `blocked` is computed from `depends` on
// every board read, and a bad edge is not a cosmetic problem:
//
//   - a dangling id blocks a ticket on a ticket that does not exist, forever;
//   - a cycle blocks every ticket in it on every other, forever;
//   - a self-dependency is the one-element case of a cycle.
//
// None of these produce an error at read time — they produce a queue that
// quietly never starts, which is far worse to debug than a rejected plan.
// Structured outputs guarantee the JSON shape; only this guarantees the graph.
func Validate(p Plan, maxTickets int) error {
	if maxTickets <= 0 {
		maxTickets = DefaultMaxTickets
	}
	if len(p.Tickets) == 0 {
		return fmt.Errorf("%w: plan contains no tickets", ErrInvalidPlan)
	}
	if len(p.Tickets) > maxTickets {
		return fmt.Errorf("%w: plan has %d tickets, limit is %d", ErrInvalidPlan, len(p.Tickets), maxTickets)
	}

	byNumber := make(map[int]Ticket, len(p.Tickets))
	for _, t := range p.Tickets {
		if t.Number <= 0 {
			return fmt.Errorf("%w: ticket %q has non-positive number %d", ErrInvalidPlan, t.Title, t.Number)
		}
		if _, dup := byNumber[t.Number]; dup {
			return fmt.Errorf("%w: two tickets share number %d", ErrInvalidPlan, t.Number)
		}
		if strings.TrimSpace(t.Title) == "" {
			return fmt.Errorf("%w: ticket %d has an empty title", ErrInvalidPlan, t.Number)
		}
		if !slices.Contains(Roles, t.Role) {
			return fmt.Errorf("%w: ticket %d has role %q, want one of %s",
				ErrInvalidPlan, t.Number, t.Role, strings.Join(Roles, ", "))
		}
		byNumber[t.Number] = t
	}

	for _, t := range p.Tickets {
		for _, dep := range t.Depends {
			if dep == t.Number {
				return fmt.Errorf("%w: ticket %d depends on itself", ErrInvalidPlan, t.Number)
			}
			if _, ok := byNumber[dep]; !ok {
				return fmt.Errorf("%w: ticket %d depends on %d, which is not in the plan", ErrInvalidPlan, t.Number, dep)
			}
		}
	}

	if cycle := findCycle(byNumber); cycle != nil {
		return fmt.Errorf("%w: dependency cycle %s", ErrInvalidPlan, formatCycle(cycle))
	}
	return nil
}

// findCycle returns one dependency cycle as a list of ticket numbers, or nil
// when the graph is acyclic. Iterative depth-first search with an explicit
// stack: a plan is small, but recursion depth driven by model output is not
// something to leave unbounded.
func findCycle(byNumber map[int]Ticket) []int {
	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	state := make(map[int]int, len(byNumber))
	parent := make(map[int]int, len(byNumber))

	// Deterministic iteration order, so the same bad plan always names the same
	// cycle — an error message that changes between identical runs is a bad
	// error message.
	starts := make([]int, 0, len(byNumber))
	for n := range byNumber {
		starts = append(starts, n)
	}
	slices.Sort(starts)

	for _, start := range starts {
		if state[start] != unvisited {
			continue
		}
		stack := []int{start}
		for len(stack) > 0 {
			n := stack[len(stack)-1]

			if state[n] == unvisited {
				state[n] = inStack
			}

			advanced := false
			deps := byNumber[n].Depends
			sorted := append([]int(nil), deps...)
			slices.Sort(sorted)
			for _, dep := range sorted {
				switch state[dep] {
				case inStack:
					return buildCycle(parent, n, dep)
				case unvisited:
					parent[dep] = n
					stack = append(stack, dep)
					advanced = true
				}
				if advanced {
					break
				}
			}
			if !advanced {
				state[n] = done
				stack = stack[:len(stack)-1]
			}
		}
	}
	return nil
}

// buildCycle walks parent pointers from `from` back to `to` to recover the
// cycle that closing edge completes.
func buildCycle(parent map[int]int, from, to int) []int {
	cycle := []int{to}
	for n := from; n != to; n = parent[n] {
		cycle = append(cycle, n)
		if len(cycle) > len(parent)+2 {
			break // defensive: malformed parent chain, report what we have
		}
	}
	slices.Reverse(cycle)
	return append(cycle, to)
}

func formatCycle(cycle []int) string {
	parts := make([]string, len(cycle))
	for i, n := range cycle {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, " -> ")
}
