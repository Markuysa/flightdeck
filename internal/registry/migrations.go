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
// projects carries only the fields core.Project exposes to callers;
// project_secrets is a distinct table so a query against projects alone —
// the shape List and Get return — can never join in a token by accident.
//
// runs records what FlightDeck itself did: "we ran routine R for ticket N at
// time T, and here is how that attempt ended". That is a fact about this
// server's own actions, unavailable from git by construction, and it is what
// lets the scheduler avoid re-firing a ticket whose branch has not appeared
// yet (ADR-007). Its `state` column is a RUN's lifecycle — running, observed,
// timed_out, failed — never a ticket's status. No table here has, or may ever
// gain, a ticket status column: ADR-001 forbids it, and TestNoStatusColumn in
// registry_test.go asserts it by introspecting this schema directly via
// sqlite_master and pragma_table_info.
const schema = `
CREATE TABLE IF NOT EXISTS projects (
	id                 TEXT PRIMARY KEY,
	name               TEXT NOT NULL,
	repo_path          TEXT NOT NULL,
	remote             TEXT NOT NULL DEFAULT '',
	owner              TEXT NOT NULL DEFAULT '',
	repo               TEXT NOT NULL DEFAULT '',
	routine_trigger_id TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS project_secrets (
	project_id    TEXT PRIMARY KEY,
	routine_token TEXT NOT NULL DEFAULT '',
	github_token  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id  TEXT NOT NULL,
	ticket_id   INTEGER NOT NULL,
	attempt     INTEGER NOT NULL,
	state       TEXT NOT NULL,
	session_url TEXT NOT NULL DEFAULT '',
	detail      TEXT NOT NULL DEFAULT '',
	started_at  TEXT NOT NULL,
	settled_at  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS runs_by_ticket ON runs (project_id, ticket_id);
CREATE INDEX IF NOT EXISTS runs_active ON runs (project_id, state);

CREATE TABLE IF NOT EXISTS agents (
	id         TEXT NOT NULL,
	project_id TEXT NOT NULL,
	name       TEXT NOT NULL,
	role       TEXT NOT NULL,
	prompt     TEXT NOT NULL DEFAULT '',
	skills     TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (project_id, id)
);

-- One agent per role per project: dispatch looks an agent up BY role, so two
-- rows sharing one would make which prompt an agent receives depend on row
-- order. The constraint is what makes that lookup deterministic.
CREATE UNIQUE INDEX IF NOT EXISTS agents_role ON agents (project_id, role);
`

// addedColumns lists columns introduced after the first released schema.
// CREATE TABLE IF NOT EXISTS is a no-op against a database created by an
// older FlightDeck, so a column added to `schema` above would silently
// never appear there — every such column must also be listed here, with
// the exact type and default it carries in `schema`.
//
// Each entry is applied with ALTER TABLE ... ADD COLUMN, guarded by a
// pragma_table_info check so re-running against an already-migrated
// database is a no-op rather than an error. This is deliberately not a
// numbered migration framework: the registry holds registration data only
// (ADR-001 forbids ticket status here), so schema change is rare and
// additive, and a column list is the whole of what that needs.
var addedColumns = []struct {
	table  string
	column string
	ddl    string
}{
	{"projects", "routine_trigger_id", "ALTER TABLE projects ADD COLUMN routine_trigger_id TEXT NOT NULL DEFAULT ''"},
}

// migrate applies schema against db, then brings an older database up to
// date with any column in addedColumns it is missing. Safe to call on an
// already-migrated database — see schema's and addedColumns' doc comments.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("applying registry schema: %w", err)
	}
	for _, c := range addedColumns {
		exists, err := columnExists(ctx, db, c.table, c.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.ExecContext(ctx, c.ddl); err != nil {
			return fmt.Errorf("adding column %s.%s: %w", c.table, c.column, err)
		}
	}
	return nil
}

// columnExists reports whether table already has column, via SQLite's
// pragma_table_info table-valued function.
func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("inspecting %s for column %s: %w", table, column, err)
	}
	return n > 0, nil
}
