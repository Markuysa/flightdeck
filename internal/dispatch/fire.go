package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// ErrDispatchFailed wraps any failure firing a project's routine: a
// transport error, a non-2xx response, or a malformed body. Per
// ARCHITECTURE.md's "External integrations" failure policy for the routine
// trigger, a dispatch failure surfaces to the user with the error and is
// never silently retried — Fire makes exactly one attempt and returns.
var ErrDispatchFailed = errors.New("dispatch: fire failed")

// ErrNoRoutineTrigger is returned by Fire when the project carries no
// RoutineTriggerID. It is a distinct sentinel, not a generic
// ErrDispatchFailed, so the API layer can answer 409 ("this project is not
// wired to a routine yet") rather than 502 ("the routine rejected us") —
// two very different things for the operator to act on.
var ErrNoRoutineTrigger = errors.New("dispatch: project has no routine trigger configured")

// fireRequest is the JSON body Fire POSTs to the trigger's /run endpoint.
// The remote-trigger API treats a run body as optional passthrough; the
// routine's own prompt is what reads these fields out of it and decides how to
// work.
//
// ticket_id names the work. The optional agent fields carry the briefing: the
// system prompt and skills configured for that ticket's role, plus any
// per-ticket note the operator added. They are omitempty because a project
// with no agents configured must send exactly the body it sent before agents
// existed — a routine reading only ticket_id keeps working untouched.
//
// FlightDeck still does not drive the routine. It states who is working, under
// what instructions; what the routine does with that is the routine's prompt.
type fireRequest struct {
	TicketID  int      `json:"ticket_id"`
	AgentName string   `json:"agent_name,omitempty"`
	Prompt    string   `json:"agent_prompt,omitempty"`
	Skills    []string `json:"agent_skills,omitempty"`
	Notes     string   `json:"ticket_notes,omitempty"`
}

// fireResponse is the subset of a trigger-run response Fire reads. The
// remote-trigger API's exact field name for the live session link is not
// pinned down by this codebase (ADR-006), so every plausible spelling is
// accepted and the first non-empty one wins — see sessionURL. A response
// carrying none of them is NOT an error: the routine was started, and a
// missing link is a missing convenience, not a failed dispatch.
type fireResponse struct {
	SessionURL string `json:"session_url"`
	URL        string `json:"url"`
	RoutineURL string `json:"routine_url"`
	WebURL     string `json:"web_url"`
	Session    struct {
		URL string `json:"url"`
	} `json:"session"`
}

// sessionURL returns the first non-empty session link spelling in r, or ""
// when the response carried none.
func (r fireResponse) sessionURL() string {
	for _, candidate := range []string{r.SessionURL, r.URL, r.RoutineURL, r.WebURL, r.Session.URL} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// triggerRunURL builds the remote-trigger run endpoint for triggerID.
// triggerID is path-escaped: it reaches here from the registry, which took
// it from an operator-filled form field, so it is untrusted input that must
// never be able to climb out of the path segment it belongs in.
func (c *Client) triggerRunURL(triggerID string) string {
	return fmt.Sprintf("%s/v1/code/triggers/%s/run",
		strings.TrimSuffix(c.routineAPIBase, "/"), url.PathEscape(triggerID))
}

// Fire implements core.Dispatcher.Fire: it runs p's Claude routine via the
// remote-trigger API, passing ticketID so the routine knows which ticket to
// implement, and returns the live session URL when the response carries one.
//
// Unlike this package's other methods, Fire consults p for more than
// owner/repo: p.RoutineTriggerID names which routine to run. The bearer
// token is the Client's own, since it is already scoped to this one
// project (see the package doc comment).
//
// On any failure — no trigger configured, building the request, the
// transport, a non-2xx status, or a malformed response body — Fire returns
// a single error and makes no further attempt.
func (c *Client) Fire(ctx context.Context, p core.Project, ticketID int, brief core.Briefing) (string, error) {
	if p.RoutineTriggerID == "" {
		return "", fmt.Errorf("%w: project %q", ErrNoRoutineTrigger, p.ID)
	}
	endpoint := c.triggerRunURL(p.RoutineTriggerID)

	body, err := json.Marshal(fireRequest{
		TicketID:  ticketID,
		AgentName: brief.AgentName,
		Prompt:    brief.Prompt,
		Skills:    brief.Skills,
		Notes:     brief.Notes,
	})
	if err != nil {
		return "", fmt.Errorf("%w: encoding request: %w", ErrDispatchFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: building request: %w", ErrDispatchFailed, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.routineToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: requesting %s: %w", ErrDispatchFailed, endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("%w: %s returned %d: %s", ErrDispatchFailed, endpoint, resp.StatusCode, respBody)
	}

	var out fireResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("%w: decoding response from %s: %w", ErrDispatchFailed, endpoint, err)
	}
	return out.sessionURL(), nil
}
