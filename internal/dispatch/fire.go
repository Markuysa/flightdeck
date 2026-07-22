package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// ErrDispatchFailed wraps any failure firing a project's routine: a
// transport error, a non-2xx response, or a malformed body. Per
// ARCHITECTURE.md's "External integrations" failure policy for the routine
// /fire endpoint, a dispatch failure surfaces to the user with the error and
// is never silently retried — Fire makes exactly one attempt and returns.
var ErrDispatchFailed = errors.New("dispatch: fire failed")

// fireRequest is the exact JSON body Fire POSTs to "<routineBaseURL>/fire".
// ticket_id is the only field the frontend/api layer needs to send — the
// routine endpoint and its bearer token already scope the request to one
// project (ARCHITECTURE.md: `POST /api/projects/{id}/dispatch` -> `{ticket_id}`).
type fireRequest struct {
	TicketID int `json:"ticket_id"`
}

// fireResponse is the exact JSON body Fire expects back. session_url is the
// field the frontend surfaces to the user immediately after dispatch
// (ARCHITECTURE.md: `{session_url}`).
type fireResponse struct {
	SessionURL string `json:"session_url"`
}

// Fire implements core.Dispatcher.Fire: it POSTs ticketID to the routine's
// /fire endpoint using the per-project routine bearer token, and returns the
// session URL the routine responds with. p is not otherwise consulted —
// this Client is already scoped to one project's routine token and base
// URL — but is part of the signature to satisfy core.Dispatcher.
//
// On any failure — building the request, the transport, a non-2xx status,
// or a malformed response body — Fire returns a single error wrapping
// ErrDispatchFailed and makes no further attempt.
func (c *Client) Fire(ctx context.Context, p core.Project, ticketID int) (string, error) {
	url := strings.TrimSuffix(c.routineBaseURL, "/") + "/fire"

	body, err := json.Marshal(fireRequest{TicketID: ticketID})
	if err != nil {
		return "", fmt.Errorf("%w: encoding request: %w", ErrDispatchFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: building request: %w", ErrDispatchFailed, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.routineToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: requesting %s: %w", ErrDispatchFailed, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("%w: %s returned %d: %s", ErrDispatchFailed, url, resp.StatusCode, respBody)
	}

	var out fireResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("%w: decoding response from %s: %w", ErrDispatchFailed, url, err)
	}
	return out.SessionURL, nil
}
