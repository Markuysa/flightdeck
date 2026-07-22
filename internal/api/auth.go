package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

// sessionCookieName is the httpOnly cookie POST /api/session sets and every
// other route accepts in place of a bearer token.
const sessionCookieName = "fd_session"

// sessionStore tracks session IDs issued by POST /api/session. In-memory
// only — sessions do not survive a restart, which is fine for FlightDeck's
// whole audience (CLAUDE.md: a single self-hosted operator) and means no
// session secret is ever written to disk.
type sessionStore struct {
	mu  sync.Mutex
	ids map[string]bool
}

func newSessionStore() *sessionStore {
	return &sessionStore{ids: make(map[string]bool)}
}

// issue mints a new random session ID, records it as valid, and returns it.
func (s *sessionStore) issue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.ids[id] = true
	s.mu.Unlock()
	return id, nil
}

func (s *sessionStore) valid(id string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ids[id]
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header, or "" when the header is absent or a different scheme.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}

// constantTimeEqual reports whether a and b are equal without leaking their
// contents through a timing side-channel — exactly the property comparing
// FLIGHTDECK_TOKEN needs (CLAUDE.md: secrets are never logged, and a naive
// == comparison leaks one matching byte at a time to a patient attacker).
// An empty a or b never matches, however s.token is configured: a server
// started with an empty token is misconfigured, not open to anyone.
func constantTimeEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// handleCreateSession implements POST /api/session: it trades a valid
// "Authorization: Bearer <FLIGHTDECK_TOKEN>" for an httpOnly session
// cookie other requests present instead of the bearer token.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if !constantTimeEqual(bearerToken(r), s.token) {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	id, err := s.sessions.issue()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	w.WriteHeader(http.StatusNoContent)
}

// requireAuth gates every route besides POST /api/session behind either a
// valid bearer token or a session cookie POST /api/session issued.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if constantTimeEqual(bearerToken(r), s.token) {
			next.ServeHTTP(w, r)
			return
		}
		if cookie, err := r.Cookie(sessionCookieName); err == nil && s.sessions.valid(cookie.Value) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}
