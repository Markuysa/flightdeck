package dispatch

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets a test act as an http.RoundTripper without a real
// network call or an httptest.Server. Shared by fire_test.go and
// merge_test.go.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// stubResponse is a canned reply for one exact "METHOD url" request key.
type stubResponse struct {
	status int
	body   string
	err    error // when set, the transport fails instead of responding
}

// newStubTransport returns a RoundTripper that answers each request by
// looking up "<method> <url>" in responses. A request to any other
// method+URL fails the test immediately — every request a method under test
// makes must be accounted for.
func newStubTransport(t *testing.T, responses map[string]stubResponse) roundTripFunc {
	t.Helper()
	return func(r *http.Request) (*http.Response, error) {
		key := r.Method + " " + r.URL.String()
		resp, ok := responses[key]
		if !ok {
			t.Fatalf("unexpected request: %s", key)
			return nil, nil
		}
		if resp.err != nil {
			return nil, resp.err
		}
		return &http.Response{
			StatusCode: resp.status,
			Body:       io.NopCloser(strings.NewReader(resp.body)),
			Header:     make(http.Header),
		}, nil
	}
}

// recordingTransport behaves like newStubTransport but also appends every
// request's "METHOD url" key to seen, regardless of whether it matched a
// canned response. TestFireAndApproveMergeAreSeparateCodePaths uses seen to
// assert exactly which requests Fire and ApproveMerge each make.
type recordingTransport struct {
	t         *testing.T
	responses map[string]stubResponse
	seen      []string
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	key := r.Method + " " + r.URL.String()
	rt.seen = append(rt.seen, key)
	resp, ok := rt.responses[key]
	if !ok {
		rt.t.Fatalf("unexpected request: %s", key)
		return nil, nil
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
	}, nil
}
