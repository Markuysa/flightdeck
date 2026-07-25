package api

import (
	"encoding/json"
	"net/http"
)

// handleSetSecrets implements PUT /api/projects/{id}/secrets: sets or
// updates the project's routine/GitHub tokens. Semantics are read-current,
// overlay, write — an omitted or empty field in the request body leaves
// that token exactly as it was, rather than blanking it, so a caller can
// rotate one token without having to resend the other. 404s when the
// project doesn't exist; 204 on success, with no body (never a token).
func (s *Server) handleSetSecrets(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}

	var body SetSecretsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	current, err := s.registry.Secrets(r.Context(), p.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load current secrets")
		return
	}
	if body.RoutineToken != "" {
		current.RoutineToken = body.RoutineToken
	}
	if body.GitHubToken != "" {
		current.GitHubToken = body.GitHubToken
	}

	if err := s.registry.SetSecrets(r.Context(), p.ID, current); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set secrets")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetSecretsStatus implements GET /api/projects/{id}/secrets: reports
// only whether each token is currently set, never the value (ADR-005). 404s
// when the project doesn't exist.
func (s *Server) handleGetSecretsStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}

	sec, err := s.registry.Secrets(r.Context(), p.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load secrets")
		return
	}
	writeJSON(w, http.StatusOK, SecretsStatus{
		RoutineTokenSet: sec.RoutineToken != "",
		GitHubTokenSet:  sec.GitHubToken != "",
	})
}
