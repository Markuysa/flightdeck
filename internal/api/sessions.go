package api

import (
	"sync"
	"time"
)

// dispatchSessionKey identifies one ticket's dispatch record.
type dispatchSessionKey struct {
	projectID string
	ticketID  int
}

// dispatchSessionInfo is what a successful dispatch records: the routine's
// session URL and the moment Fire returned it.
type dispatchSessionInfo struct {
	sessionURL   string
	dispatchedAt time.Time
}

// dispatchSessionStore remembers the session URL and dispatch time of every
// ticket this server has fired, keyed by (project id, ticket id), so
// GET /api/agents can link back to a live routine session (ticket 019).
//
// In-memory only, by design: a live session is ephemeral, and CLAUDE.md /
// ADR-001 keep the durable store (SQLite) to local UI config — never ticket
// or session state. Forgetting every entry on restart is therefore correct,
// not a bug: a session URL is neither ticket status (which must never be
// persisted) nor a secret, but it still doesn't belong in the durable store.
//
// Safe for concurrent use: handleDispatch records from one request
// goroutine while handleListAgents looks up from others, all at once.
type dispatchSessionStore struct {
	mu   sync.RWMutex
	byID map[dispatchSessionKey]dispatchSessionInfo
}

func newDispatchSessionStore() *dispatchSessionStore {
	return &dispatchSessionStore{byID: make(map[dispatchSessionKey]dispatchSessionInfo)}
}

// Record remembers that ticketID in projectID was dispatched at "at" and its
// routine session lives at url. Callers record only after Fire succeeds
// (handleDispatch); a failed dispatch never reaches here.
func (s *dispatchSessionStore) Record(projectID string, ticketID int, url string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[dispatchSessionKey{projectID, ticketID}] = dispatchSessionInfo{sessionURL: url, dispatchedAt: at}
}

// Lookup returns the recorded dispatch info for (projectID, ticketID), if
// this server has ever dispatched it. An in_progress ticket whose branch was
// not dispatched through this server (fired by hand, or before this server
// last started) simply has no entry — handleListAgents leaves its
// session_url empty rather than fabricating one.
func (s *dispatchSessionStore) Lookup(projectID string, ticketID int) (dispatchSessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.byID[dispatchSessionKey{projectID, ticketID}]
	return info, ok
}
