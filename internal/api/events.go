package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// EventKind names one of the SSE events GET /api/events emits — exactly
// EVENT_KINDS in ui/src/lib/sse.ts.
type EventKind string

const (
	EventBoardChanged    EventKind = "board.changed"
	EventDispatchStarted EventKind = "dispatch.started"
	EventCIChanged       EventKind = "ci.changed"
)

// Event is one message a Broker fans out: Kind names the SSE "event:" line,
// Data marshals to the "data:" line's JSON — matching the frontend's
// FlightDeckEvent{kind, data} (ui/src/lib/sse.ts).
type Event struct {
	Kind EventKind
	Data any
}

// Broker fans out published events to every subscribed SSE client. It is
// in-process only — FlightDeck has no external queue (ARCHITECTURE.md:
// "Store: none for domain data") — and safe for concurrent Publish/
// Subscribe from multiple goroutines.
type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewBroker returns an empty Broker.
func NewBroker() *Broker {
	return &Broker{subs: make(map[chan Event]struct{})}
}

// Subscribe registers a new subscriber and returns its channel and a cancel
// function the caller must call exactly once (typically deferred) to
// unregister and close it.
func (b *Broker) Subscribe() (ch chan Event, cancel func()) {
	ch = make(chan Event, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
	}
}

// Publish fans data out to every current subscriber as an Event of kind. A
// subscriber whose buffer is already full drops the event rather than
// blocking every other subscriber and publisher — SSE here is a live
// nice-to-have with no queue to replay from, not a guaranteed-delivery log
// (ponytail: a bounded buffer plus drop-when-full is the deliberate
// simplification; a slow-consumer backpressure scheme would need one if a
// consumer ever depended on never missing an event).
func (b *Broker) Publish(kind EventKind, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- Event{Kind: kind, Data: data}:
		default:
		}
	}
}

// writeSSE writes ev to w in the wire format ui/src/lib/sse.ts's
// EventSource listeners expect: a named "event:" line, a JSON "data:" line,
// and the blank line terminating the message, then flushes so the client
// sees it immediately.
func writeSSE(w io.Writer, flusher http.Flusher, ev Event) error {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// handleEvents implements GET /api/events: a text/event-stream subscription
// that stays open until the client disconnects or the request context is
// canceled, forwarding every Broker.Publish call as a named SSE event.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	// Subscribe before writing any bytes: the header flush below is what a
	// client's response read unblocks on, so by the time a caller observes
	// this response, this subscription is guaranteed to already exist — no
	// event published after that point is ever missed (see events_test.go).
	ch, cancel := s.events.Subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := writeSSE(w, flusher, ev); err != nil {
				return
			}
		}
	}
}
