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
// v1 sourcing note (deviation documented in ticket 008's handoff):
// SessionURL is always "" — no source records a live routine session once
// dispatched, in v1. StartedAt and LastActivityAt are both filled with the
// ticket's branch's tip commit time (best-effort; distinguishing "when
// this branch started" from "its last activity" would require walking
// full history against main, out of scope for v1).
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
	return agent
}
