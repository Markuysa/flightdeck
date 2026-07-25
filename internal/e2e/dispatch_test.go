package e2e

import (
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestDispatchDisabledForNonReadyTicketOverHTTP is PRD §6's third acceptance
// criterion, first half: "Dispatch is disabled for a ticket that is not
// ready." Over the real HTTP API, dispatching a blocked ticket is rejected
// (409) and the stubbed routine (core.Dispatcher.Fire) is never called.
func TestDispatchDisabledForNonReadyTicketOverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Unmet dependency", Role: "backend", Status: "todo"},
			{ID: 2, Title: "Blocked ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		nil,
	)

	store := newTempStore(t)
	dispatcher := newStubDispatcher()
	h := newHarness(t, store, api.NewGitHubSource(store), newStubDispatcherFactory(dispatcher))
	project := h.registerProject("Dispatch Fixture", repo.Path, nil)

	resp := h.do(http.MethodPost, "/api/projects/"+project.ID+"/dispatch", api.DispatchRequest{TicketID: 2})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("dispatch of a blocked ticket = %d, want 409: %s", resp.StatusCode, bodyString(t, resp))
	}
	if len(dispatcher.fireCalls) != 0 {
		t.Errorf("Fire was called %+v times for a non-ready ticket, want zero", dispatcher.fireCalls)
	}
}

// TestDispatchFiresReadyTicketWithCorrectPayloadOverHTTP is PRD §6's third
// acceptance criterion, second half: "Dispatching a ticket issues the
// routine /fire call and surfaces the returned session URL." Over the real
// HTTP API, dispatching a ready ticket calls the stubbed routine
// (core.Dispatcher.Fire) with exactly that ticket's id and project, and the
// response body surfaces the session URL Fire returned.
func TestDispatchFiresReadyTicketWithCorrectPayloadOverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		nil,
	)

	store := newTempStore(t)
	dispatcher := newStubDispatcher()
	dispatcher.sessionURL = "https://routines.example.com/sessions/fixture-abc123"
	h := newHarness(t, store, api.NewGitHubSource(store), newStubDispatcherFactory(dispatcher))
	project := h.registerProject("Dispatch Fixture", repo.Path, nil)

	resp := h.do(http.MethodPost, "/api/projects/"+project.ID+"/dispatch", api.DispatchRequest{TicketID: 2})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch of a ready ticket = %d, want 200: %s", resp.StatusCode, bodyString(t, resp))
	}
	got := decodeJSON[api.DispatchResponse](t, resp)
	if got.SessionURL != dispatcher.sessionURL {
		t.Errorf("session_url = %q, want %q (the stubbed routine's canned URL)", got.SessionURL, dispatcher.sessionURL)
	}

	if len(dispatcher.fireCalls) != 1 {
		t.Fatalf("fireCalls = %+v, want exactly one call", dispatcher.fireCalls)
	}
	call := dispatcher.fireCalls[0]
	if call.ProjectID != project.ID || call.TicketID != 2 {
		t.Errorf("Fire call = %+v, want {ProjectID: %q, TicketID: 2}", call, project.ID)
	}
}
