package api

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDispatchSessionStoreLookupMissReturnsFalse(t *testing.T) {
	t.Parallel()
	s := newDispatchSessionStore()

	if _, ok := s.Lookup("acme", 1); ok {
		t.Fatal("Lookup on an empty store returned ok=true, want false")
	}
}

func TestDispatchSessionStoreRecordThenLookup(t *testing.T) {
	t.Parallel()
	s := newDispatchSessionStore()
	at := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)

	s.Record("acme", 1, "https://routines.example.com/sessions/abc", at)

	info, ok := s.Lookup("acme", 1)
	if !ok {
		t.Fatal("Lookup after Record returned ok=false, want true")
	}
	if info.sessionURL != "https://routines.example.com/sessions/abc" || !info.dispatchedAt.Equal(at) {
		t.Errorf("info = %+v, want sessionURL/dispatchedAt as recorded", info)
	}

	// A different project or ticket id is a distinct key.
	if _, ok := s.Lookup("beta", 1); ok {
		t.Error("Lookup for a different project id returned ok=true, want false")
	}
	if _, ok := s.Lookup("acme", 2); ok {
		t.Error("Lookup for a different ticket id returned ok=true, want false")
	}
}

func TestDispatchSessionStoreRecordOverwritesPriorEntry(t *testing.T) {
	t.Parallel()
	s := newDispatchSessionStore()
	first := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	second := time.Date(2026, 7, 25, 11, 0, 0, 0, time.UTC)

	s.Record("acme", 1, "https://routines.example.com/sessions/first", first)
	s.Record("acme", 1, "https://routines.example.com/sessions/second", second)

	info, ok := s.Lookup("acme", 1)
	if !ok {
		t.Fatal("Lookup returned ok=false, want true")
	}
	if info.sessionURL != "https://routines.example.com/sessions/second" || !info.dispatchedAt.Equal(second) {
		t.Errorf("info = %+v, want the second Record to win", info)
	}
}

// TestDispatchSessionStoreConcurrentAccess exercises the store the way the
// server actually does — one goroutine per project recording its own
// dispatch while many others read every key — under -race. Each writer
// touches only its own key, so the outcome is deterministic: no assertion
// depends on goroutine scheduling order.
func TestDispatchSessionStoreConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := newDispatchSessionStore()
	const projects = 8
	at := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for i := 0; i < projects; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "project-" + strconv.Itoa(i)
			s.Record(id, i, "https://routines.example.com/sessions/"+id, at)
		}(i)
	}
	// Concurrent readers race against the writers above; they only assert
	// that Lookup never panics/deadlocks, not on any particular ordering.
	for i := 0; i < projects; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "project-" + strconv.Itoa(i)
			s.Lookup(id, i)
		}(i)
	}
	wg.Wait()

	for i := 0; i < projects; i++ {
		id := "project-" + strconv.Itoa(i)
		info, ok := s.Lookup(id, i)
		if !ok {
			t.Fatalf("Lookup(%q, %d) after all writers finished = ok=false, want true", id, i)
		}
		if info.sessionURL != "https://routines.example.com/sessions/"+id {
			t.Errorf("Lookup(%q, %d).sessionURL = %q, want the recorded URL", id, i, info.sessionURL)
		}
	}
}
