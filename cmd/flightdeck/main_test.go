package main

import "testing"

// serve() itself is exercised end to end by internal/app's tests (config
// sourcing, lifecycle, graceful shutdown); this only covers run()'s own
// argument handling, since that is the part cmd/flightdeck owns. None of
// these cases reach runServe's call into serve() — every one either errors
// during argument parsing or takes a branch (version, help) that returns
// before serve would ever be invoked, so nothing here boots a real listener.
func TestRun_ArgumentHandling(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: nil, wantErr: true},
		{name: "too many args", args: []string{"serve", "extra"}, wantErr: true},
		{name: "serve with unknown flag", args: []string{"serve", "--bogus"}, wantErr: true},
		{name: "unknown command", args: []string{"bogus"}, wantErr: true},
		{name: "version", args: []string{"version"}, wantErr: false},
		{name: "version with extra args", args: []string{"version", "extra"}, wantErr: true},
		{name: "help", args: []string{"--help"}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("run(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestParseServeArgs covers `serve`'s own flag parsing directly, since
// runServe's success path calls the real (blocking) serve() and can't be
// exercised by a unit test.
func TestParseServeArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDemo bool
		wantErr  bool
	}{
		{name: "no flags", args: nil, wantDemo: false, wantErr: false},
		{name: "--demo", args: []string{"--demo"}, wantDemo: true, wantErr: false},
		{name: "unknown flag", args: []string{"--bogus"}, wantErr: true},
		{name: "unexpected positional argument", args: []string{"extra"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			demoMode, err := parseServeArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseServeArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil && demoMode != tt.wantDemo {
				t.Errorf("parseServeArgs(%v) demoMode = %v, want %v", tt.args, demoMode, tt.wantDemo)
			}
		})
	}
}
