package registry

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

// Test tokens are deliberately fake and short — CI's secrets job greps for
// credential-shaped strings (ghp_<36>, sk-ant-..., sk-<32>), and a realistic
// token literal anywhere, even in a test, would trip it.
const (
	testRoutineToken = "routine-tok-test"
	testGitHubToken  = "gh-tok-test"
)

func testProject(id string) core.Project {
	return core.Project{
		ID:       id,
		Name:     "Test Project",
		RepoPath: "/repos/" + id,
		Remote:   "github",
		Owner:    "acme",
		Repo:     id,
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestRoundTripPersistsAcrossReopen is ticket 006's acceptance criterion 3:
// a store round-trip against a REAL temp file (t.TempDir(), never
// ":memory:") — open, write, close, reopen, read back — proving both
// persistence and that migrate is idempotent against an existing database.
func TestRoundTripPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "registry.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	p := testProject("acme-web")
	if err := store.Add(ctx, p); err != nil {
		t.Fatalf("Add: %v", err)
	}
	sec := Secrets{RoutineToken: testRoutineToken, GitHubToken: testGitHubToken}
	if err := store.SetSecrets(ctx, p.ID, sec); err != nil {
		t.Fatalf("SetSecrets: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen the same file: migrate must be a no-op against the existing
	// schema, and the data written before Close must still be there.
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopening %s: %v", path, err)
	}
	defer func() { _ = reopened.Close() }()

	got, err := reopened.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got != p {
		t.Fatalf("Get after reopen = %+v, want %+v", got, p)
	}

	gotSec, err := reopened.Secrets(ctx, p.ID)
	if err != nil {
		t.Fatalf("Secrets after reopen: %v", err)
	}
	if gotSec != sec {
		t.Fatal("Secrets after reopen did not match what was stored before Close")
	}
}

func TestGetMissingReturnsErrProjectNotFound(t *testing.T) {
	store := openTestStore(t)

	_, err := store.Get(context.Background(), "nope")
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Fatalf("Get(missing) error = %v, want core.ErrProjectNotFound", err)
	}
}

func TestRemoveMissingReturnsErrProjectNotFound(t *testing.T) {
	store := openTestStore(t)

	err := store.Remove(context.Background(), "nope")
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Fatalf("Remove(missing) error = %v, want core.ErrProjectNotFound", err)
	}
}

func TestSecretsMissingProjectReturnsErrProjectNotFound(t *testing.T) {
	store := openTestStore(t)

	_, err := store.Secrets(context.Background(), "nope")
	if !errors.Is(err, core.ErrProjectNotFound) {
		t.Fatalf("Secrets(missing) error = %v, want core.ErrProjectNotFound", err)
	}
}

func TestSecretsDefaultToZeroValueWhenUnset(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	p := testProject("acme-web")
	if err := store.Add(ctx, p); err != nil {
		t.Fatalf("Add: %v", err)
	}

	sec, err := store.Secrets(ctx, p.ID)
	if err != nil {
		t.Fatalf("Secrets: %v", err)
	}
	if sec != (Secrets{}) {
		t.Fatalf("Secrets for a project with none set = %+v, want the zero value", sec)
	}
}

func TestListAndRemove(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	web, api := testProject("acme-web"), testProject("acme-api")
	for _, p := range []core.Project{web, api} {
		if err := store.Add(ctx, p); err != nil {
			t.Fatalf("Add(%s): %v", p.ID, err)
		}
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].ID != "acme-api" || got[1].ID != "acme-web" {
		t.Fatalf("List = %+v, want [acme-api, acme-web]", got)
	}

	if err := store.Remove(ctx, web.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List after Remove: %v", err)
	}
	if len(got) != 1 || got[0].ID != api.ID {
		t.Fatalf("List after Remove = %+v, want only %s", got, api.ID)
	}

	// Removing a project also removes its secrets: re-adding the same ID
	// must not resurrect a stale token.
	if err := store.SetSecrets(ctx, api.ID, Secrets{RoutineToken: testRoutineToken}); err != nil {
		t.Fatalf("SetSecrets: %v", err)
	}
	if err := store.Remove(ctx, api.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := store.Add(ctx, api); err != nil {
		t.Fatalf("re-Add: %v", err)
	}
	sec, err := store.Secrets(ctx, api.ID)
	if err != nil {
		t.Fatalf("Secrets after re-Add: %v", err)
	}
	if sec != (Secrets{}) {
		t.Fatalf("Secrets survived Remove: %+v, want the zero value", sec)
	}
}

// TestSecretsRedactedInStringAndGoString guards CLAUDE.md's rule that
// tokens are never logged: an accidental fmt.Sprintf("%v"/"%+v"/"%#v", sec)
// — the shape a stray log line or error wrap takes — must not print either
// token.
func TestSecretsRedactedInStringAndGoString(t *testing.T) {
	sec := Secrets{RoutineToken: testRoutineToken, GitHubToken: testGitHubToken}
	reprs := []string{
		sec.String(),
		fmt.Sprintf("%v", sec),
		fmt.Sprintf("%+v", sec),
		fmt.Sprintf("%#v", sec),
	}
	for _, r := range reprs {
		if strings.Contains(r, sec.RoutineToken) || strings.Contains(r, sec.GitHubToken) {
			t.Fatalf("Secrets representation leaked a token: %q", r)
		}
	}
}

// TestNoStatusColumn is ticket 006's acceptance criterion 4 (ADR-001): it
// introspects the live schema — every table from sqlite_master, then every
// column from pragma_table_info — and fails if any column name contains
// "status". Nothing about ticket status may ever be persisted here.
func TestNoStatusColumn(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	rows, err := store.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		t.Fatalf("listing tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing tables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no tables found — expected the registry schema to exist")
	}

	for _, table := range tables {
		func() {
			colRows, err := store.db.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info(%q)`, table))
			if err != nil {
				t.Fatalf("introspecting table %q: %v", table, err)
			}
			defer func() { _ = colRows.Close() }()

			for colRows.Next() {
				var col string
				if err := colRows.Scan(&col); err != nil {
					t.Fatalf("scanning column name for table %q: %v", table, err)
				}
				if strings.Contains(strings.ToLower(col), "status") {
					t.Fatalf("table %q has column %q — ADR-001 forbids storing ticket status anywhere", table, col)
				}
			}
			if err := colRows.Err(); err != nil {
				t.Fatalf("introspecting table %q: %v", table, err)
			}
		}()
	}
}

// TestNoOtherInternalImports asserts ADR-004: internal/registry imports
// core and nothing else internal, matching the dependency rule
// docs/ARCHITECTURE.md sets for every feature package.
func TestNoOtherInternalImports(t *testing.T) {
	const (
		forbiddenPrefix = "github.com/Markuysa/flightdeck/internal/"
		allowedInternal = "github.com/Markuysa/flightdeck/internal/core"
	)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquoting import path in %s: %v", name, err)
			}
			if strings.HasPrefix(path, forbiddenPrefix) && path != allowedInternal {
				t.Fatalf("internal/registry must import no internal package besides core (ADR-004): %s imports %q", name, path)
			}
		}
	}
}
