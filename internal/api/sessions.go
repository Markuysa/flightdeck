package api

import (
	"context"
	"time"

	"github.com/Markuysa/flightdeck/internal/registry"
)

// recordDispatch persists a dispatch this server just fired, numbering it as
// the ticket's next attempt, and returns the stored run.
//
// This replaced an in-memory map (which forgot every session link on restart)
// with a durable row, for two reasons that are worth keeping straight:
//
//   - a link to a live agent session should survive a server restart; the
//     session outlives the process that started it;
//   - the background scheduler decides what to fire partly from these rows.
//     A human dispatch that left no trace would still read `ready` until its
//     branch appeared, and the scheduler would fire the very same ticket
//     again. Writing here is what makes the two dispatch paths agree.
//
// ADR-001 is not in tension with this: a run says "this server fired routine R
// for ticket N at time T", which no amount of reading git can reconstruct. It
// is never the ticket's status, which is still derived on every read.
func (s *Server) recordDispatch(ctx context.Context, projectID string, ticketID int, sessionURL string, at time.Time) (registry.Run, error) {
	attempt := 1
	if last, found, err := s.runs.LatestRun(ctx, projectID, ticketID); err == nil && found {
		attempt = last.Attempt + 1
	}
	return s.runs.StartRun(ctx, projectID, ticketID, attempt, sessionURL, at)
}

// lastDispatch returns the most recent run recorded for a ticket, if any.
// A ticket whose branch was never dispatched through this server — fired by
// hand, or by a FlightDeck pointed at a different database — simply has no
// run, and callers leave its session_url empty rather than fabricating one.
func (s *Server) lastDispatch(ctx context.Context, projectID string, ticketID int) (registry.Run, bool) {
	run, found, err := s.runs.LatestRun(ctx, projectID, ticketID)
	if err != nil || !found {
		return registry.Run{}, false
	}
	return run, true
}
