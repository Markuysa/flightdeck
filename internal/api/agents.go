package api

import (
	"context"
	"net/http"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

// handleListAgents implements GET /api/agents (US-4): every ticket
// currently in_progress across every registered project — a claude/NNN-*
// branch whose file still says todo is an agent working right now.
//
// SessionURL, StartedAt, and LastActivityAt sourcing (ticket 019 closes the
// gap ticket 008's handoff documented): LastActivityAt always comes from the
// branch tip's commit time (BranchCommitTime) — the most recent commit is
// the most honest "last activity" signal available without a real heartbeat.
// StartedAt defaults to that same commit time, but is overridden with the
// server's own dispatchSessions-recorded dispatch time when this server is
// the one that fired the ticket — that timestamp is more accurate than the
// branch's tip commit time for when the session itself began. SessionURL
// stays "" unless this server dispatched the ticket: agents whose branch
// was not dispatched through this server (fired by hand, or before this
// server's last restart — the store is in-memory only) simply have no URL,
// which is honest, not fabricated.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, err := s.registry.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	agents := []AgentSession{}
	for _, p := range projects {
		tickets, err := s.source.BoardTickets(ctx, p)
		if err != nil {
			// A project whose checkout can't be read right now is skipped,
			// not fatal to the whole endpoint — a missing/unreadable repo
			// degrades, it never crashes what other projects can show
			// (ARCHITECTURE.md's failure policy).
			continue
		}
		for _, t := range tickets {
			if t.Status != core.StatusInProgress {
				continue
			}
			agents = append(agents, s.agentSession(ctx, p, t))
		}
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) agentSession(ctx context.Context, p core.Project, t core.BoardTicket) AgentSession {
	agent := AgentSession{
		ProjectID:   p.ID,
		ProjectName: p.Name,
		TicketID:    t.ID,
		TicketTitle: t.Title,
		Branch:      t.Branch,
	}
	if ts, err := s.source.BranchCommitTime(ctx, p, t.Branch); err == nil {
		iso := ts.Format(time.RFC3339)
		agent.StartedAt = iso
		agent.LastActivityAt = iso
	}
	if info, ok := s.dispatchSessions.Lookup(p.ID, t.ID); ok {
		agent.SessionURL = info.sessionURL
		agent.StartedAt = info.dispatchedAt.Format(time.RFC3339)
	}
	return agent
}
