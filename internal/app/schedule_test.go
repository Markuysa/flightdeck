package app

import (
	"testing"
	"time"

	"github.com/Markuysa/flightdeck/internal/schedule"
)

// TestScheduleIsDisabledUnlessAskedFor is the safety default this whole file
// exists for. Every other FLIGHTDECK_* variable treats empty as "use a
// sensible default"; this one must not, because its default would spend agent
// budget without anyone asking (ADR-007).
func TestScheduleIsDisabledUnlessAskedFor(t *testing.T) {
	for _, raw := range []string{"", "   ", "off", "OFF", "0", "0s", "-5m"} {
		t.Run("value "+raw, func(t *testing.T) {
			got, err := parseScheduleInterval(raw)
			if err != nil {
				t.Fatalf("parseScheduleInterval(%q): %v", raw, err)
			}
			if got != 0 {
				t.Errorf("parseScheduleInterval(%q) = %s, want 0 (disabled)", raw, got)
			}
		})
	}
}

func TestScheduleIntervalParsesDurations(t *testing.T) {
	got, err := parseScheduleInterval("30s")
	if err != nil {
		t.Fatalf("parseScheduleInterval: %v", err)
	}
	if got != 30*time.Second {
		t.Errorf("parseScheduleInterval(30s) = %s, want 30s", got)
	}
}

func TestScheduleIntervalRejectsGarbage(t *testing.T) {
	if _, err := parseScheduleInterval("soon"); err == nil {
		t.Error("parseScheduleInterval(\"soon\") = nil error, want a startup failure")
	}
}

// TestScheduleConfigFromEnvDefaults: unset knobs fall back to the package's
// conservative defaults rather than to zero, which would mean "no parallelism,
// no timeout, no attempts".
func TestScheduleConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("FLIGHTDECK_SCHEDULE_INTERVAL", "1m")
	cfg, err := scheduleConfigFromEnv()
	if err != nil {
		t.Fatalf("scheduleConfigFromEnv: %v", err)
	}
	if cfg.Interval != time.Minute {
		t.Errorf("Interval = %s, want 1m", cfg.Interval)
	}
	if cfg.MaxParallel != schedule.DefaultMaxParallel {
		t.Errorf("MaxParallel = %d, want %d", cfg.MaxParallel, schedule.DefaultMaxParallel)
	}
	if cfg.RunTimeout != schedule.DefaultRunTimeout {
		t.Errorf("RunTimeout = %s, want %s", cfg.RunTimeout, schedule.DefaultRunTimeout)
	}
	if cfg.MaxAttempts != schedule.DefaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", cfg.MaxAttempts, schedule.DefaultMaxAttempts)
	}
}

func TestScheduleConfigFromEnvReadsOverrides(t *testing.T) {
	t.Setenv("FLIGHTDECK_SCHEDULE_INTERVAL", "10s")
	t.Setenv("FLIGHTDECK_MAX_PARALLEL", "5")
	t.Setenv("FLIGHTDECK_RUN_TIMEOUT", "45m")
	t.Setenv("FLIGHTDECK_MAX_ATTEMPTS", "3")

	cfg, err := scheduleConfigFromEnv()
	if err != nil {
		t.Fatalf("scheduleConfigFromEnv: %v", err)
	}
	want := schedule.Config{Interval: 10 * time.Second, MaxParallel: 5, RunTimeout: 45 * time.Minute, MaxAttempts: 3}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

// TestScheduleConfigFailsFastOnBadValues: a typo must never silently fall back
// to a default that spends money.
func TestScheduleConfigFailsFastOnBadValues(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"parallelism not a number", "FLIGHTDECK_MAX_PARALLEL", "lots"},
		{"parallelism zero", "FLIGHTDECK_MAX_PARALLEL", "0"},
		{"parallelism negative", "FLIGHTDECK_MAX_PARALLEL", "-1"},
		{"timeout not a duration", "FLIGHTDECK_RUN_TIMEOUT", "ages"},
		{"timeout zero", "FLIGHTDECK_RUN_TIMEOUT", "0"},
		{"attempts not a number", "FLIGHTDECK_MAX_ATTEMPTS", "many"},
		{"attempts zero", "FLIGHTDECK_MAX_ATTEMPTS", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := scheduleConfigFromEnv(); err == nil {
				t.Errorf("%s=%q was accepted, want a startup failure", tc.key, tc.value)
			}
		})
	}
}
