// Package app is FlightDeck's composition root (ADR-004): it is the only
// package that imports every feature package's concrete implementation and
// wires them into one running server. internal/api, internal/registry,
// internal/dispatch, internal/source/git, internal/source/github and
// internal/webui each depend only on internal/core (and stdlib); nothing
// among them imports another. Only this package knows the whole graph.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/registry"
	"github.com/Markuysa/flightdeck/internal/webui"
)

const (
	// defaultAddr is FLIGHTDECK_ADDR's default: one port for the API and
	// the embedded UI (ADR-002).
	defaultAddr = ":8080"
	// defaultDBPath is FLIGHTDECK_DB's default, relative to the working
	// directory flightdeck is started from.
	defaultDBPath = "flightdeck.db"
	// shutdownTimeout bounds how long Run waits for in-flight requests to
	// finish once asked to stop, before giving up on a clean shutdown.
	shutdownTimeout = 10 * time.Second
)

// Config is FlightDeck's runtime configuration, sourced from the
// environment by ConfigFromEnv.
type Config struct {
	// Token is FLIGHTDECK_TOKEN: the bearer token the UI trades for a
	// session cookie (docs/ARCHITECTURE.md's auth flow). Required — a
	// server with no token cannot authenticate anyone.
	Token string
	// Addr is the listen address for both the API and the embedded UI,
	// e.g. ":8080" or "127.0.0.1:8080".
	Addr string
	// DBPath is the registry's SQLite file path (gitignored FlightDeck
	// runtime state — never ticket status, ADR-001).
	DBPath string
}

// ConfigFromEnv builds a Config from the process environment:
//   - FLIGHTDECK_TOKEN — required; ConfigFromEnv fails fast when it is unset
//     or empty, since no request could ever authenticate against an empty
//     token (api.constantTimeEqual itself never matches an empty value).
//   - FLIGHTDECK_ADDR — optional, defaults to ":8080".
//   - FLIGHTDECK_DB — optional, defaults to "flightdeck.db".
func ConfigFromEnv() (Config, error) {
	token := os.Getenv("FLIGHTDECK_TOKEN")
	if token == "" {
		return Config{}, errors.New("FLIGHTDECK_TOKEN is required (set it to a random secret before running flightdeck serve)")
	}

	addr := os.Getenv("FLIGHTDECK_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	dbPath := os.Getenv("FLIGHTDECK_DB")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	return Config{Token: token, Addr: addr, DBPath: dbPath}, nil
}

// App is a fully wired FlightDeck server: the registry, the git+github
// backed API source, the dispatcher factory, the API itself, and the
// embedded UI, all mounted on one handler. Build one with New; it owns the
// registry's lifecycle until Close.
type App struct {
	cfg     Config
	store   *registry.Store
	apiSrv  *api.Server
	handler http.Handler
}

// New builds an App from cfg: it opens the registry, wires the real
// api.ProjectSource (git + github + derive) and DispatcherFactory over it,
// constructs the API server, and mounts the embedded UI alongside it — the
// entire object graph docs/ARCHITECTURE.md describes, assembled in the one
// place ADR-004 allows it.
func New(cfg Config) (*App, error) {
	if cfg.Token == "" {
		return nil, errors.New("app: Config.Token is required")
	}
	addr := cfg.Addr
	if addr == "" {
		addr = defaultAddr
	}
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = defaultDBPath
	}
	cfg.Addr, cfg.DBPath = addr, dbPath

	store, err := registry.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("app: opening registry at %s: %w", cfg.DBPath, err)
	}

	apiSrv := api.NewServer(api.Config{
		Token:      cfg.Token,
		Registry:   store,
		Source:     api.NewGitHubSource(store),
		Dispatcher: api.NewDispatcherFactory(store),
	})

	mux := http.NewServeMux()
	mux.Handle("/api/", apiSrv.Handler())
	mux.Handle("/", webui.Handler())

	return &App{cfg: cfg, store: store, apiSrv: apiSrv, handler: mux}, nil
}

// Handler returns the App's full http.Handler — the API under /api/ and the
// embedded UI everywhere else — for mounting behind a real listener (Run)
// or driving directly in tests (httptest.NewServer, httptest.NewRequest).
func (a *App) Handler() http.Handler { return a.handler }

// Events returns the API server's event broker, for a caller that wants to
// Publish board.changed/ci.changed from outside a request (docs/ARCHITECTURE.md
// notes no background publisher is wired yet; ticket 008's handoff).
func (a *App) Events() *api.Broker { return a.apiSrv.Events() }

// Close releases the App's resources: the registry's database connection.
func (a *App) Close() error { return a.store.Close() }

// Run serves the App on cfg.Addr until ctx is canceled, then shuts down
// gracefully: it stops accepting new connections, gives in-flight requests
// up to shutdownTimeout to finish, and returns. ctx is typically derived
// from signal.NotifyContext, so an operator's Ctrl-C or a SIGTERM triggers
// this path rather than killing the process mid-request. Run does not close
// the registry — call (*App).Close after Run returns, so a caller that
// wants to log or report the final error can still do so with the registry
// available.
func (a *App) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.cfg.Addr)
	if err != nil {
		return fmt.Errorf("app: listening on %s: %w", a.cfg.Addr, err)
	}

	srv := &http.Server{Handler: a.handler}

	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("app: shutting down server: %w", err)
	}
	return <-serveErr
}
