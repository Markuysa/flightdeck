package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/registry"
	"github.com/Markuysa/flightdeck/internal/schedule"
)

// runStoreAdapter presents registry.Store as a schedule.RunStore.
//
// The two types describe the same rows but neither may import the other:
// internal/schedule imports only internal/core (ADR-004), so it declares its
// own Run and Outcome rather than reaching for registry's. Translating between
// them is the composition root's job, which is this file — the same role
// internal/app already plays for every other concrete wiring.
type runStoreAdapter struct{ store *registry.Store }

var _ schedule.RunStore = runStoreAdapter{}

// toScheduleRun narrows a registry.Run to the fields the scheduler reads.
func toScheduleRun(r registry.Run) schedule.Run {
	return schedule.Run{
		ID:         r.ID,
		TicketID:   r.TicketID,
		Attempt:    r.Attempt,
		SessionURL: r.SessionURL,
		StartedAt:  r.StartedAt,
	}
}

// toRunState maps the scheduler's outcome vocabulary onto the store's. An
// unrecognised outcome is an error rather than a silent default: writing the
// wrong terminal state would misreport whether work actually started.
func toRunState(o schedule.Outcome) (registry.RunState, error) {
	switch o {
	case schedule.Observed:
		return registry.RunObserved, nil
	case schedule.TimedOut:
		return registry.RunTimedOut, nil
	case schedule.Failed:
		return registry.RunFailed, nil
	default:
		return "", fmt.Errorf("app: unknown run outcome %q", o)
	}
}

func (a runStoreAdapter) ActiveRuns(ctx context.Context, projectID string) ([]schedule.Run, error) {
	stored, err := a.store.ActiveRuns(ctx, projectID)
	if err != nil {
		return nil, err
	}
	runs := make([]schedule.Run, len(stored))
	for i, r := range stored {
		runs[i] = toScheduleRun(r)
	}
	return runs, nil
}

func (a runStoreAdapter) LatestRun(ctx context.Context, projectID string, ticketID int) (schedule.Run, bool, bool, error) {
	stored, found, err := a.store.LatestRun(ctx, projectID, ticketID)
	if err != nil || !found {
		return schedule.Run{}, false, false, err
	}
	return toScheduleRun(stored), stored.Active(), true, nil
}

func (a runStoreAdapter) StartRun(ctx context.Context, projectID string, ticketID, attempt int, sessionURL string, at time.Time) (schedule.Run, error) {
	stored, err := a.store.StartRun(ctx, projectID, ticketID, attempt, sessionURL, at)
	if err != nil {
		return schedule.Run{}, err
	}
	return toScheduleRun(stored), nil
}

func (a runStoreAdapter) Settle(ctx context.Context, runID int64, outcome schedule.Outcome, detail string, at time.Time) error {
	state, err := toRunState(outcome)
	if err != nil {
		return err
	}
	return a.store.SettleRun(ctx, runID, state, detail, at)
}

// brokerPublisher presents *api.Broker as a schedule.Publisher.
//
// The mismatch is one named type: the broker's Publish takes an
// api.EventKind, and internal/schedule cannot name that type without
// importing internal/api (ADR-004), so it declares its event names as plain
// strings. Converting here is a one-line cost for keeping the scheduler
// independent of the HTTP layer — and it means the scheduler's
// dispatch.started reaches the exact same SSE stream a human dispatch does.
type brokerPublisher struct{ broker *api.Broker }

var _ schedule.Publisher = brokerPublisher{}

func (p brokerPublisher) Publish(kind string, data map[string]any) {
	p.broker.Publish(api.EventKind(kind), data)
}

// briefingAdapter presents registry.Store as both api.AgentReader and
// schedule.BriefingReader, so a ticket dispatched by a human and the same
// ticket dispatched by the scheduler carry identical instructions.
//
// The scheduler's side returns a value rather than an error: a briefing is
// decoration on a dispatch, and refusing to start work because the decoration
// could not be loaded would trade a working feature for a broken one. A miss
// yields the zero Briefing, which is exactly the pre-agents behaviour.
type briefingAdapter struct{ store *registry.Store }

var _ schedule.BriefingReader = briefingAdapter{}

func (b briefingAdapter) Briefing(ctx context.Context, projectID, role string) core.Briefing {
	if role == "" {
		return core.Briefing{}
	}
	agent, err := b.store.AgentForRole(ctx, projectID, role)
	if err != nil {
		if !errors.Is(err, registry.ErrAgentNotFound) {
			log.Printf("flightdeck: schedule: project %q role %q: reading agent: %v", projectID, role, err)
		}
		return core.Briefing{}
	}
	return core.Briefing{AgentName: agent.Name, Prompt: agent.Prompt, Skills: agent.Skills}
}
