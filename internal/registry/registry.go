// Package registry is the one thing FlightDeck legitimately persists: which
// projects are registered, and their secrets (ADR-001 forbids storing
// anything else here — no ticket status, ever, see TestNoStatusColumn in
// registry_test.go). It is backed by SQLite via modernc.org/sqlite (pure
// Go, no cgo), so the binary keeps its single-binary shape (ADR-002).
//
// core.Project — the API-facing DTO — carries no secret fields by design
// (see internal/core/project.go). A project's routine token and GitHub
// token live in a separate table and are reachable only through Store's
// Secrets and SetSecrets methods, never through Add, List or Get: nothing
// that returns a core.Project can leak a token, so a handler that forwards
// one to the browser is safe by construction (ADR-005).
package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Markuysa/flightdeck/internal/core"
)

// Store is the registry: a SQLite-backed set of registered projects and
// their secrets. Construct one with Open.
type Store struct {
	db *sql.DB
}

// Secrets is a project's routine dispatch token and GitHub token — the two
// tokens ADR-005 says never reach the browser. It is reachable only through
// Store.Secrets and Store.SetSecrets, never through Add, List or Get, and
// it is never embedded in core.Project.
//
// String and GoString are overridden so an accidental %v, %+v or %#v on a
// Secrets value — in a log line, an error, a debugger — prints neither
// token.
type Secrets struct {
	RoutineToken string
	GitHubToken  string
}

// String implements fmt.Stringer, redacting both tokens.
func (Secrets) String() string { return "registry.Secrets{redacted}" }

// GoString implements fmt.GoStringer, redacting both tokens under %#v too.
func (Secrets) GoString() string { return "registry.Secrets{redacted}" }

// Add registers p. p.ID must be unique among registered projects; adding an
// ID that already exists returns the underlying UNIQUE constraint error.
func (s *Store) Add(ctx context.Context, p core.Project) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, repo_path, remote, owner, repo)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.RepoPath, p.Remote, p.Owner, p.Repo)
	if err != nil {
		return fmt.Errorf("adding project %q: %w", p.ID, err)
	}
	return nil
}

// List returns every registered project, ordered by ID. It returns an
// empty slice, never an error, when no project is registered.
func (s *Store) List(ctx context.Context) ([]core.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, repo_path, remote, owner, repo FROM projects ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	projects := []core.Project{}
	for rows.Next() {
		var p core.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.RepoPath, &p.Remote, &p.Owner, &p.Repo); err != nil {
			return nil, fmt.Errorf("scanning project row: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	return projects, nil
}

// Get returns the registered project with id, or a wrapped
// core.ErrProjectNotFound (errors.Is-detectable) when it does not exist.
func (s *Store) Get(ctx context.Context, id string) (core.Project, error) {
	var p core.Project
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, repo_path, remote, owner, repo FROM projects WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.RepoPath, &p.Remote, &p.Owner, &p.Repo)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return core.Project{}, fmt.Errorf("getting project %q: %w", id, core.ErrProjectNotFound)
	case err != nil:
		return core.Project{}, fmt.Errorf("getting project %q: %w", id, err)
	}
	return p, nil
}

// Remove deletes the registered project with id and any secrets stored for
// it, or returns a wrapped core.ErrProjectNotFound when id is not
// registered. Both deletes happen in one transaction, so a failure removing
// secrets never leaves the project row deleted with orphaned tokens intact,
// or vice versa.
func (s *Store) Remove(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("removing project %q: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("removing project %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("removing project %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("removing project %q: %w", id, core.ErrProjectNotFound)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_secrets WHERE project_id = ?`, id); err != nil {
		return fmt.Errorf("removing secrets for project %q: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("removing project %q: %w", id, err)
	}
	return nil
}

// Secrets returns the routine and GitHub tokens stored for projectID. A
// registered project with no tokens set yet returns a zero-value Secrets
// and a nil error; an unregistered projectID returns a wrapped
// core.ErrProjectNotFound. This is the only path a token takes out of the
// store — Add, List and Get never touch project_secrets (ADR-005).
func (s *Store) Secrets(ctx context.Context, projectID string) (Secrets, error) {
	var sec Secrets
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(s.routine_token, ''), COALESCE(s.github_token, '')
		FROM projects p
		LEFT JOIN project_secrets s ON s.project_id = p.id
		WHERE p.id = ?
	`, projectID).Scan(&sec.RoutineToken, &sec.GitHubToken)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Secrets{}, fmt.Errorf("getting secrets for project %q: %w", projectID, core.ErrProjectNotFound)
	case err != nil:
		return Secrets{}, fmt.Errorf("getting secrets for project %q: %w", projectID, err)
	}
	return sec, nil
}

// SetSecrets stores sec as projectID's routine and GitHub tokens,
// overwriting any previous value.
func (s *Store) SetSecrets(ctx context.Context, projectID string, sec Secrets) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_secrets (project_id, routine_token, github_token)
		VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			routine_token = excluded.routine_token,
			github_token  = excluded.github_token
	`, projectID, sec.RoutineToken, sec.GitHubToken)
	if err != nil {
		return fmt.Errorf("setting secrets for project %q: %w", projectID, err)
	}
	return nil
}
