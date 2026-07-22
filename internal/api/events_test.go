package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestWriteSSEFormatsNamedEventWithJSONData checks the exact wire format
// ui/src/lib/sse.ts's EventSource listeners parse: an "event: <kind>" line
// immediately followed by a "data: <json>" line and a blank line.
func TestWriteSSEFormatsNamedEventWithJSONData(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind EventKind
		data any
		want string
	}{
		{EventBoardChanged, map[string]string{"project_id": "acme"}, `event: board.changed
data: {"project_id":"acme"}

`},
		{EventDispatchStarted, map[string]any{"ticket_id": 7}, `event: dispatch.started
data: {"ticket_id":7}

`},
		{EventCIChanged, map[string]string{"branch": "claude/004-x"}, `event: ci.changed
data: {"branch":"claude/004-x"}

`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		if err := writeSSE(rec, rec, Event{Kind: tc.kind, Data: tc.data}); err != nil {
			t.Fatalf("writeSSE(%s): %v", tc.kind, err)
		}
		if got := rec.Body.String(); got != tc.want {
			t.Errorf("writeSSE(%s) wrote %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestBrokerPublishReachesSubscriber is a pure, deterministic (no
// goroutines, no network) proof that Publish delivers to a channel a prior
// Subscribe call returned, carrying the right Kind and Data.
func TestBrokerPublishReachesSubscriber(t *testing.T) {
	t.Parallel()
	b := NewBroker()
	ch, cancel := b.Subscribe()
	defer cancel()

	b.Publish(EventCIChanged, map[string]string{"branch": "claude/004-x"})

	ev := <-ch // buffered channel: Publish above already queued it.
	if ev.Kind != EventCIChanged {
		t.Errorf("event kind = %q, want %q", ev.Kind, EventCIChanged)
	}
	data, ok := ev.Data.(map[string]string)
	if !ok || data["branch"] != "claude/004-x" {
		t.Errorf("event data = %+v, want {branch: claude/004-x}", ev.Data)
	}
}

func TestBrokerPublishWithNoSubscribersDoesNotBlock(t *testing.T) {
	t.Parallel()
	b := NewBroker()
	b.Publish(EventBoardChanged, nil) // must return, not block/panic
}

// TestEventsSSEDeliversEachEventKindOverHTTP proves the full wiring: a real
// HTTP client connected to GET /api/events observes each Broker.Publish
// call as a distinct SSE message with the correct event: name and JSON
// data:. handleEvents subscribes before flushing its response headers
// (events.go's doc comment), so once this test's http.Get returns having
// read the headers, the subscription is guaranteed to exist — no sleep or
// retry loop is needed to avoid the publish racing the subscribe.
func TestEventsSSEDeliversEachEventKindOverHTTP(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	server := httptest.NewServer(http.HandlerFunc(ts.srv.handleEvents))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connecting to SSE endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/events = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	broker := ts.srv.Events()
	broker.Publish(EventBoardChanged, map[string]string{"project_id": "acme"})
	broker.Publish(EventDispatchStarted, map[string]any{"ticket_id": 7, "session_url": "https://x/y"})
	broker.Publish(EventCIChanged, map[string]string{"branch": "claude/004-x"})

	reader := bufio.NewReader(resp.Body)
	wantEvents := []struct {
		event string
		data  string
	}{
		{"board.changed", `{"project_id":"acme"}`},
		{"dispatch.started", `{"session_url":"https://x/y","ticket_id":7}`},
		{"ci.changed", `{"branch":"claude/004-x"}`},
	}
	for _, want := range wantEvents {
		eventLine := readNonEmptyLine(t, reader)
		dataLine := readNonEmptyLine(t, reader)
		if eventLine != "event: "+want.event {
			t.Errorf("event line = %q, want %q", eventLine, "event: "+want.event)
		}
		if dataLine != "data: "+want.data {
			t.Errorf("data line = %q, want %q", dataLine, "data: "+want.data)
		}
	}
}

func readNonEmptyLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("reading SSE stream: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		if line != "" {
			return line
		}
	}
}
