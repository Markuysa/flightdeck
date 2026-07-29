// schedule.go reads the autonomous dispatcher's configuration from the
// environment. It is deliberately separate from the refresher's parsing next
// door: the refresher defaults to ON because observing is free, and the
// scheduler defaults to OFF because dispatching is not (ADR-007).
package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Markuysa/flightdeck/internal/schedule"
)

// scheduleConfigFromEnv builds the scheduler's Config:
//
//   - FLIGHTDECK_SCHEDULE_INTERVAL — how often to look for work to start.
//     UNSET, empty, "off", or a duration <= 0 all mean DISABLED. This is the
//     one FLIGHTDECK_* variable where empty does not mean "use a sensible
//     default": a server that dispatches on its own must be asked to.
//   - FLIGHTDECK_MAX_PARALLEL — tickets in flight per project (default 2).
//   - FLIGHTDECK_RUN_TIMEOUT — how long a fired run may go without its branch
//     appearing before it is retried (default 30m).
//   - FLIGHTDECK_MAX_ATTEMPTS — dispatches per ticket before giving up and
//     leaving it for a human (default 2).
//
// Every value that fails to parse is a configuration error returned to the
// caller, which fails startup — the same fail-fast contract the rest of
// ConfigFromEnv uses. A typo must never silently fall back to a default that
// spends money.
func scheduleConfigFromEnv() (schedule.Config, error) {
	interval, err := parseScheduleInterval(os.Getenv("FLIGHTDECK_SCHEDULE_INTERVAL"))
	if err != nil {
		return schedule.Config{}, err
	}
	maxParallel, err := parsePositiveInt("FLIGHTDECK_MAX_PARALLEL", os.Getenv("FLIGHTDECK_MAX_PARALLEL"), schedule.DefaultMaxParallel)
	if err != nil {
		return schedule.Config{}, err
	}
	runTimeout, err := parsePositiveDuration("FLIGHTDECK_RUN_TIMEOUT", os.Getenv("FLIGHTDECK_RUN_TIMEOUT"), schedule.DefaultRunTimeout)
	if err != nil {
		return schedule.Config{}, err
	}
	maxAttempts, err := parsePositiveInt("FLIGHTDECK_MAX_ATTEMPTS", os.Getenv("FLIGHTDECK_MAX_ATTEMPTS"), schedule.DefaultMaxAttempts)
	if err != nil {
		return schedule.Config{}, err
	}

	return schedule.Config{
		Interval:    interval,
		MaxParallel: maxParallel,
		RunTimeout:  runTimeout,
		MaxAttempts: maxAttempts,
	}, nil
}

// parseScheduleInterval turns FLIGHTDECK_SCHEDULE_INTERVAL into a tick
// interval, where absent means disabled (see scheduleConfigFromEnv).
func parseScheduleInterval(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "off") {
		return 0, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid FLIGHTDECK_SCHEDULE_INTERVAL %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, nil
	}
	return d, nil
}

func parsePositiveInt(name, raw string, fallback int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, raw, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be greater than zero", name, raw)
	}
	return n, nil
}

func parsePositiveDuration(name, raw string, fallback time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be greater than zero", name, raw)
	}
	return d, nil
}
