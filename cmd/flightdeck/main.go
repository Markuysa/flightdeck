// Command flightdeck is FlightDeck's single binary: it serves the REST +
// SSE API and the embedded UI from one port (ADR-002).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Markuysa/flightdeck/internal/app"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "dev"

// demoToken is FLIGHTDECK_TOKEN's default in `serve --demo`, used only when
// the operator hasn't set one — so a newcomer can run the demo with no
// environment at all. It is printed to stdout at startup so they can still
// authenticate; the non-demo path never defaults a token
// (app.ConfigFromEnv keeps failing fast when FLIGHTDECK_TOKEN is unset).
const demoToken = "flightdeck-demo-token"

const usage = `flightdeck is the CEO console for a team of coding agents.

Usage:
  flightdeck serve [--demo]   Serve the API and the embedded UI on one port.
  flightdeck version          Print the flightdeck version.

Environment (serve):
  FLIGHTDECK_TOKEN   required — the bearer token clients authenticate with.
                     In --demo mode, defaults to a known dev token when unset.
  FLIGHTDECK_ADDR    optional — listen address, default ":8080".
  FLIGHTDECK_DB      optional — registry SQLite file path, default "flightdeck.db".
                     In --demo mode, defaults to a throwaway temp file when unset.

Flags (serve):
  --demo   Seed a fixture project (tickets across several derived statuses,
           no real git remote required) so the UI is explorable with
           nothing set up.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "flightdeck:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return errors.New("expected a command")
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "version":
		if len(args) != 1 {
			fmt.Fprint(os.Stderr, usage)
			return errors.New("version takes no arguments")
		}
		fmt.Println(version)
		return nil
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// parseServeArgs parses `serve`'s own arguments — currently just --demo —
// separately from runServe so tests can exercise the parsing without
// booting the real server (serve blocks until it is interrupted).
func parseServeArgs(args []string) (demoMode bool, err error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	demoFlag := fs.Bool("demo", false, "seed a demo project and start with a known dev token if unset")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return *demoFlag, nil
}

func runServe(args []string) error {
	demoMode, err := parseServeArgs(args)
	if err != nil {
		fmt.Fprint(os.Stderr, usage)
		return err
	}
	return serve(demoMode)
}

// serve builds the composition root from the environment and runs it until
// an operator interrupts it (Ctrl-C) or the process receives SIGTERM, then
// shuts down gracefully. In demo mode it fills in FLIGHTDECK_TOKEN and
// FLIGHTDECK_DB when the operator left them unset, and seeds the demo
// project (internal/demo) before serving.
func serve(demoMode bool) error {
	if demoMode {
		applyDemoDefaults()
	}

	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return err
	}

	a, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := a.Close(); err != nil {
			log.Printf("flightdeck: closing registry: %v", err)
		}
	}()

	if demoMode {
		project, err := a.SeedDemo(context.Background())
		if err != nil {
			return fmt.Errorf("seeding demo project: %w", err)
		}
		fmt.Printf("flightdeck: demo project %q ready at %s (open /p/%s)\n", project.Name, project.RepoPath, project.ID)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("flightdeck %s listening on %s", version, cfg.Addr)
	return a.Run(ctx)
}

// applyDemoDefaults fills FLIGHTDECK_TOKEN and FLIGHTDECK_DB from --demo's
// defaults when the operator left them unset, printing the token to stdout
// so `flightdeck serve --demo` works with no environment configured at all.
// It never overrides a value the operator already set.
func applyDemoDefaults() {
	if os.Getenv("FLIGHTDECK_TOKEN") == "" {
		_ = os.Setenv("FLIGHTDECK_TOKEN", demoToken)
		fmt.Printf("flightdeck: demo mode — using token %q (set FLIGHTDECK_TOKEN to override)\n", demoToken)
	}
	if os.Getenv("FLIGHTDECK_DB") == "" {
		_ = os.Setenv("FLIGHTDECK_DB", filepath.Join(os.TempDir(), "flightdeck-demo.db"))
	}
}
