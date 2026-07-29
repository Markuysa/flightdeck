// agents.go persists per-project agent configuration: which specialist takes
// which role of ticket, and under what instructions.
//
// Like runs.go, this is FlightDeck's own state rather than derived state — an
// agent's prompt exists nowhere in git and cannot be recomputed from it, so
// ADR-001 has nothing to say about storing it. Nothing here is a ticket status.
package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Markuysa/flightdeck/internal/core"
)

// ErrAgentNotFound is returned when no agent matches the requested id or role.
var ErrAgentNotFound = errors.New("registry: agent not found")

// skillSeparator joins the skills list into its single stored column. Skills
// are short operator-chosen identifiers, so a newline-delimited list is enough
// and stays readable in a database browser — a JSON blob would buy nothing and
// make the column hostile to inspect by hand.
const skillSeparator = "\n"

func encodeSkills(skills []string) string {
	cleaned := make([]string, 0, len(skills))
	for _, s := range skills {
		// A skill containing a newline would decode as two skills, so the
		// separator is stripped rather than trusted.
		if s = strings.TrimSpace(strings.ReplaceAll(s, skillSeparator, " ")); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return strings.Join(cleaned, skillSeparator)
}

func decodeSkills(stored string) []string {
	if stored == "" {
		return []string{}
	}
	parts := strings.Split(stored, skillSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SetAgent stores a, replacing any agent already configured for its project and
// role. Replace-by-role rather than insert is deliberate: the operator's mental
// model is "the backend agent", singular, and an accumulating list of stale
// backend agents would make dispatch's role lookup ambiguous.
func (s *Store) SetAgent(ctx context.Context, a core.Agent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saving agent %q: %w", a.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear whatever held this role before, including an agent under a
	// different id — otherwise the unique index rejects the insert and the
	// operator sees a constraint error instead of their edit taking effect.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM agents WHERE project_id = ? AND role = ?`, a.ProjectID, a.Role); err != nil {
		return fmt.Errorf("saving agent %q: %w", a.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agents (id, project_id, name, role, prompt, skills)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, id) DO UPDATE SET
			name = excluded.name, role = excluded.role,
			prompt = excluded.prompt, skills = excluded.skills
	`, a.ID, a.ProjectID, a.Name, a.Role, a.Prompt, encodeSkills(a.Skills)); err != nil {
		return fmt.Errorf("saving agent %q: %w", a.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("saving agent %q: %w", a.ID, err)
	}
	return nil
}

// Agents returns every agent configured for projectID, ordered by role.
func (s *Store) Agents(ctx context.Context, projectID string) ([]core.Agent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, role, prompt, skills
		FROM agents WHERE project_id = ? ORDER BY role
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing agents for project %q: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()

	agents := []core.Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning agent row: %w", err)
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing agents for project %q: %w", projectID, err)
	}
	return agents, nil
}

// AgentForRole returns the agent configured to take role in projectID, or a
// wrapped ErrAgentNotFound. Dispatch calls this on every fire; a miss is the
// normal case for a project that has configured no agents, not an error worth
// failing the dispatch over.
func (s *Store) AgentForRole(ctx context.Context, projectID, role string) (core.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, role, prompt, skills
		FROM agents WHERE project_id = ? AND role = ?
	`, projectID, role)

	a, err := scanAgent(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return core.Agent{}, fmt.Errorf("role %q in project %q: %w", role, projectID, ErrAgentNotFound)
	case err != nil:
		return core.Agent{}, fmt.Errorf("reading agent for role %q: %w", role, err)
	}
	return a, nil
}

// RemoveAgent deletes projectID's agent with id.
func (s *Store) RemoveAgent(ctx context.Context, projectID, id string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agents WHERE project_id = ? AND id = ?`, projectID, id)
	if err != nil {
		return fmt.Errorf("removing agent %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("removing agent %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("agent %q in project %q: %w", id, projectID, ErrAgentNotFound)
	}
	return nil
}

func scanAgent(sc rowScanner) (core.Agent, error) {
	var (
		a      core.Agent
		skills string
	)
	if err := sc.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Role, &a.Prompt, &skills); err != nil {
		return core.Agent{}, err
	}
	a.Skills = decodeSkills(skills)
	return a, nil
}

// removeAgents deletes projectID's agents. Called from Remove, inside the same
// transaction that drops the project.
func removeAgents(ctx context.Context, tx *sql.Tx, projectID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM agents WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("removing agents for project %q: %w", projectID, err)
	}
	return nil
}
