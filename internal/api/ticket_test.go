package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

func TestGetTicketIncludesDependsDetail(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Foundation"}, Status: core.StatusDone},
		{Ticket: core.Ticket{ID: 2, Title: "Builds on 1", Depends: []int{1}}, Status: core.StatusReady},
	})

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/tickets/2", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET ticket = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	detail := decodeJSON[TicketDetail](t, rec)

	if detail.ID != 2 || detail.Title != "Builds on 1" {
		t.Fatalf("detail = %+v, want ticket 2", detail)
	}
	if len(detail.DependsDetail) != 1 || detail.DependsDetail[0].ID != 1 {
		t.Fatalf("depends_detail = %+v, want a single entry for ticket 1", detail.DependsDetail)
	}
	if detail.DependsDetail[0].Status != core.StatusDone {
		t.Errorf("depends_detail[0].Status = %q, want done", detail.DependsDetail[0].Status)
	}
}

func TestGetTicketUnknownTicketReturns404(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
	})

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/tickets/999", nil, ts.token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unknown ticket = %d, want 404", rec.Code)
	}
}

func TestGetTicketInvalidIDReturns400(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/tickets/not-a-number", nil, ts.token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET ticket with non-numeric id = %d, want 400", rec.Code)
	}
}
