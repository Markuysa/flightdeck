package main

import "testing"

// serve() itself is exercised end to end by internal/app's tests (config
// sourcing, lifecycle, graceful shutdown); this only covers run()'s own
// argument handling, since that is the part cmd/flightdeck owns.
func TestRun_ArgumentHandling(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: nil, wantErr: true},
		{name: "too many args", args: []string{"serve", "extra"}, wantErr: true},
		{name: "unknown command", args: []string{"bogus"}, wantErr: true},
		{name: "version", args: []string{"version"}, wantErr: false},
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
