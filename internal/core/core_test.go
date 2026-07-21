package core

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestTicketHasNoStatusField asserts ADR-001: status is derived, never
// parsed as truth from the ticket file, so Ticket must carry no Status
// field.
func TestTicketHasNoStatusField(t *testing.T) {
	if _, ok := reflect.TypeFor[Ticket]().FieldByName("Status"); ok {
		t.Fatalf("core.Ticket must not have a Status field (ADR-001): status is derived, never stored")
	}
}

// TestNoInternalImports asserts ADR-004: internal/core imports no other
// internal package, keeping it the dependency-free root every feature
// package builds on. It parses this package's own non-test sources and
// fails if any import points back into internal/.
func TestNoInternalImports(t *testing.T) {
	const forbiddenPrefix = "github.com/Markuysa/flightdeck/internal/"

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
			if strings.HasPrefix(path, forbiddenPrefix) {
				t.Fatalf("internal/core must import no other internal package (ADR-004): %s imports %q", name, path)
			}
		}
	}
}
