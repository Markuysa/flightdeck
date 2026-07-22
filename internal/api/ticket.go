package api

import (
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
)

// handleGetTicket implements GET /api/projects/{id}/tickets/{tid} (US-3):
// the ticket's own derived state plus each dependency's own BoardTicket, so
// the UI can render handoffs and the dependency chain without extra
// round-trips.
func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	tid, ok := ticketIDOr400(w, r)
	if !ok {
		return
	}

	tickets, err := s.source.BoardTickets(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute project board")
		return
	}
	byID := make(map[int]core.BoardTicket, len(tickets))
	for _, t := range tickets {
		byID[t.ID] = t
	}
	ticket, ok := byID[tid]
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	dependsDetail := make([]core.BoardTicket, 0, len(ticket.Depends))
	for _, dep := range ticket.Depends {
		if d, ok := byID[dep]; ok {
			dependsDetail = append(dependsDetail, d)
		}
	}

	writeJSON(w, http.StatusOK, TicketDetail{BoardTicket: ticket, DependsDetail: dependsDetail})
}
