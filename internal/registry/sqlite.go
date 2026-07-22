// sqlite.go opens the registry's SQLite file and wires the pure-Go
// modernc.org/sqlite driver — the pinned choice (ticket 006): no cgo, so
// cross-compiling and the single-binary story (ADR-002) both stay intact.
package registry

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver name Open uses
)

// Open opens (creating if necessary) the SQLite database at path and
// applies the registry's schema. path is caller-supplied and gitignored —
// this package has no opinion on where it lives beyond that it must name a
// real file: the round-trip this store exists for only holds if the file
// survives process restarts, so tests exercise a t.TempDir() file, never
// ":memory:".
//
// Open is safe to call repeatedly against the same path: migrate's
// statements are all CREATE TABLE IF NOT EXISTS, so reopening an existing
// database is a no-op against its schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening registry database at %s: %w", path, err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to registry database at %s: %w", path, err)
	}
	if err := migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
