package api

import (
	"context"
	"log"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// AgentReader resolves which agent takes a given role in a project.
// registry.Store satisfies it structurally.
type AgentReader interface {
	AgentForRole(ctx context.Context, projectID, role string) (core.Agent, error)
}

// briefingFor assembles what a dispatch tells the routine beyond the ticket id:
// the agent configured for this ticket's role, plus the operator's per-ticket
// note.
//
// Every part is optional and a miss is never an error. A project with no agents
// configured, an unreadable agent store, or a role nobody has staffed all
// produce an empty briefing and a dispatch identical to the one FlightDeck sent
// before agents existed. Failing a dispatch because the *decoration* could not
// be loaded would trade a working feature for a broken one.
func (s *Server) briefingFor(ctx context.Context, p core.Project, t core.BoardTicket, notes string) core.Briefing {
	brief := core.Briefing{Notes: notes}

	if s.agents == nil || t.Role == "" {
		return brief
	}
	agent, err := s.agents.AgentForRole(ctx, p.ID, t.Role)
	if err != nil {
		// A missing agent for this role is the ordinary case and is not worth a
		// log line on every dispatch; anything else is worth knowing about, but
		// still not worth failing over.
		if !isAgentNotFound(err) {
			log.Printf("flightdeck: api: project %q ticket %d: reading agent for role %q: %v",
				p.ID, t.ID, t.Role, err)
		}
		return brief
	}

	brief.AgentName = agent.Name
	brief.Prompt = agent.Prompt
	brief.Skills = agent.Skills
	return brief
}

// isAgentNotFound reports whether err is the registry's "no agent for this
// role" sentinel. It matches by string rather than errors.Is because
// internal/api may not import internal/registry's error just to classify a
// non-fatal miss — and the only consequence of a wrong answer here is one
// extra log line.
func isAgentNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "agent not found")
}
