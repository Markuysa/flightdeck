// Command flightdeck is FlightDeck's single binary: it serves the REST +
// SSE API and the embedded UI from one port (ADR-002).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Markuysa/flightdeck/internal/app"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `flightdeck is the CEO console for a team of coding agents.

Usage:
  flightdeck serve     Serve the API and the embedded UI on one port.
  flightdeck version    Print the flightdeck version.

Environment (serve):
  FLIGHTDECK_TOKEN   required — the bearer token clients authenticate with.
  FLIGHTDECK_ADDR    optional — listen address, default ":8080".
  FLIGHTDECK_DB      optional — registry SQLite file path, default "flightdeck.db".
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "flightdeck:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, usage)
		return errors.New("expected exactly one command")
	}

	switch args[0] {
	case "serve":
		return serve()
	case "version":
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

// serve builds the composition root from the environment and runs it until
// an operator interrupts it (Ctrl-C) or the process receives SIGTERM, then
// shuts down gracefully.
func serve() error {
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("flightdeck %s listening on %s", version, cfg.Addr)
	return a.Run(ctx)
}
