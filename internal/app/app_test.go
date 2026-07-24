package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigFromEnv_RequiresToken(t *testing.T) {
	t.Setenv("FLIGHTDECK_TOKEN", "")
	t.Setenv("FLIGHTDECK_ADDR", "")
	t.Setenv("FLIGHTDECK_DB", "")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("ConfigFromEnv with no FLIGHTDECK_TOKEN = nil error, want an error")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("FLIGHTDECK_TOKEN", "s3cret")
	t.Setenv("FLIGHTDECK_ADDR", "")
	t.Setenv("FLIGHTDECK_DB", "")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if cfg.Token != "s3cret" {
		t.Errorf("Token = %q, want %q", cfg.Token, "s3cret")
	}
	if cfg.Addr != defaultAddr {
		t.Errorf("Addr = %q, want default %q", cfg.Addr, defaultAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Errorf("DBPath = %q, want default %q", cfg.DBPath, defaultDBPath)
	}
}

func TestConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("FLIGHTDECK_TOKEN", "s3cret")
	t.Setenv("FLIGHTDECK_ADDR", "127.0.0.1:9090")
	t.Setenv("FLIGHTDECK_DB", "/tmp/custom.db")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Errorf("Addr = %q, want the FLIGHTDECK_ADDR override", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/custom.db" {
		t.Errorf("DBPath = %q, want the FLIGHTDECK_DB override", cfg.DBPath)
	}
}

func TestNew_RequiresToken(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	_, err := New(Config{Token: "", Addr: ":0", DBPath: dbPath})
	if err == nil {
		t.Fatal("New with an empty Token = nil error, want an error")
	}
}

func TestNew_MountsAPIAndUIOnOneHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	a, err := New(Config{Token: "s3cret", Addr: ":0", DBPath: dbPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Close() }()

	h := a.Handler()

	// /api/session is served by the API mount and rejects a missing token.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/session", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/session with no auth = %d, want 401 (API not mounted?)", rec.Code)
	}

	// Any other path falls through to the embedded UI.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agents", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /agents = %d, want 200 from the embedded UI (SPA fallback)", rec.Code)
	}
}

func TestApp_RunServesUntilContextCanceledThenShutsDownGracefully(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flightdeck.db")
	a, err := New(Config{Token: "s3cret", Addr: "127.0.0.1:0", DBPath: dbPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v after graceful shutdown, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s of ctx being canceled (shutdown hung)")
	}
}
