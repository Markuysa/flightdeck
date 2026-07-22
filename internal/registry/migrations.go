// migrations.go holds the registry's schema, embedded directly in the
// binary as a Go string constant so there is no separate migrations
// directory to ship or locate at runtime.
package registry

import (
	"context"
	"database/sql"
	"fmt"
)

// schema is applied in full on every Open. Every statement is CREATE TABLE
// IF NOT EXISTS, which is what makes re-running it against a database that
// already has these tables a no-op (the "idempotent on open" requirement)
// without needing a separate schema-version check.
//
// Two tables, deliberately: projects carries only the fields core.Project
// exposes to callers; project_secrets is a distinct table so a query
// against projects alone — the shape List and Get return — can never join
// in a token by accident. Neither table has, or may ever gain, a status
// column: ADR-001 forbids storing ticket status anywhere, and
// TestNoStatusColumn in registry_test.go asserts it by introspecting this
// schema directly via sqlite_master and pragma_table_info.
const schema = `
CREATE TABLE IF NOT EXISTS projects (
	id        TEXT PRIMARY KEY,
	name      TEXT NOT NULL,
	repo_path TEXT NOT NULL,
	remote    TEXT NOT NULL DEFAULT '',
	owner     TEXT NOT NULL DEFAULT '',
	repo      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS project_secrets (
	project_id    TEXT PRIMARY KEY,
	routine_token TEXT NOT NULL DEFAULT '',
	github_token  TEXT NOT NULL DEFAULT ''
);
`

// migrate applies schema against db. Safe to call on an already-migrated
// database — see schema's doc comment.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("applying registry schema: %w", err)
	}
	return nil
}
