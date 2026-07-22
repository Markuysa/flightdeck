package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

// TestCIStateBuckets is table-driven over every bucket ciState can produce
// and the precedence between them, per the mapping documented on ciState.
func TestCIStateBuckets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		runs []checkRun
		want string
	}{
		{
			name: "no checks at all",
			runs: nil,
			want: ciUnknown,
		},
		{
			name: "all successful",
			runs: []checkRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "neutral"},
				{Status: "completed", Conclusion: "skipped"},
			},
			want: ciGreen,
		},
		{
			name: "one still queued",
			runs: []checkRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "queued"},
			},
			want: ciPending,
		},
		{
			name: "one in progress",
			runs: []checkRun{
				{Status: "in_progress"},
			},
			want: ciPending,
		},
		{
			name: "one failure among passing checks",
			runs: []checkRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "failure"},
			},
			want: ciRed,
		},
		{
			name: "timed out",
			runs: []checkRun{{Status: "completed", Conclusion: "timed_out"}},
			want: ciRed,
		},
		{
			name: "cancelled",
			runs: []checkRun{{Status: "completed", Conclusion: "cancelled"}},
			want: ciRed,
		},
		{
			name: "action required",
			runs: []checkRun{{Status: "completed", Conclusion: "action_required"}},
			want: ciRed,
		},
		{
			name: "failure takes precedence over a still-running check",
			runs: []checkRun{
				{Status: "in_progress"},
				{Status: "completed", Conclusion: "failure"},
			},
			want: ciRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body, err := json.Marshal(checkRunsResponse{TotalCount: len(tt.runs), CheckRuns: tt.runs})
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			const sha = "deadbeef"
			url := apiBaseURL + "/repos/acme/widgets/commits/" + sha + "/check-runs"
			c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{
				Transport: newStubTransport(t, map[string]stubResponse{url: {status: 200, body: string(body)}}),
			}))

			got, err := c.ciState(context.Background(), sha)
			if err != nil {
				t.Fatalf("ciState(%q): %v", sha, err)
			}
			if got != tt.want {
				t.Errorf("ciState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCIStateAPIErrorIsTyped(t *testing.T) {
	t.Parallel()
	const sha = "deadbeef"
	url := apiBaseURL + "/repos/acme/widgets/commits/" + sha + "/check-runs"
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{
		Transport: newStubTransport(t, map[string]stubResponse{url: {status: 503, body: `{"message": "service unavailable"}`}}),
	}))

	_, err := c.ciState(context.Background(), sha)
	if err == nil {
		t.Fatal("ciState() error = nil, want ErrGitHubUnavailable")
	}
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("ciState() error = %v, want it to wrap ErrGitHubUnavailable", err)
	}
}
