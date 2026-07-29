package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// jsonResponse builds a 200 response with an optional ETag.
func jsonResponse(body, etag string) *http.Response {
	h := make(http.Header)
	if etag != "" {
		h.Set("ETag", etag)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: h}
}

const prListBody = `[{"number":7,"html_url":"https://github.com/acme/widgets/pull/7","head":{"ref":"claude/007-x","sha":"abc"}}]`

// TestConditionalRequestReusesCachedBodyOn304 is the whole point of the
// cache: the second read must send If-None-Match and, on GitHub's 304,
// produce the same board without the body being re-sent. GitHub does not
// charge a 304 against the REST rate limit, which is what makes a 5s
// refresher affordable.
func TestConditionalRequestReusesCachedBodyOn304(t *testing.T) {
	t.Parallel()
	var sentIfNoneMatch []string
	var calls int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		sentIfNoneMatch = append(sentIfNoneMatch, r.Header.Get("If-None-Match"))
		if strings.Contains(r.URL.Path, "/commits/") {
			return jsonResponse(`{"state":"success"}`, ""), nil
		}
		if r.Header.Get("If-None-Match") == `W/"etag-1"` {
			return &http.Response{StatusCode: http.StatusNotModified, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return jsonResponse(prListBody, `W/"etag-1"`), nil
	})

	cache := NewCache()
	newClient := func() *Client {
		return New("acme", "widgets", "tok",
			WithHTTPClient(&http.Client{Transport: transport}), WithCache(cache))
	}

	first, err := newClient().OpenPRs(context.Background())
	if err != nil {
		t.Fatalf("first OpenPRs: %v", err)
	}

	// A NEW client, as internal/api builds per board read — the cache is
	// what has to survive, not the client.
	second, err := newClient().OpenPRs(context.Background())
	if err != nil {
		t.Fatalf("second OpenPRs: %v", err)
	}

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("OpenPRs returned %d then %d PRs, want 1 each", len(first), len(second))
	}
	if first["claude/007-x"] != second["claude/007-x"] {
		t.Errorf("304 produced a different PR state:\n first  = %+v\n second = %+v",
			first["claude/007-x"], second["claude/007-x"])
	}
	if sentIfNoneMatch[0] != "" {
		t.Errorf("first request sent If-None-Match %q, want none", sentIfNoneMatch[0])
	}
	var revalidated bool
	for _, v := range sentIfNoneMatch[1:] {
		if v == `W/"etag-1"` {
			revalidated = true
		}
	}
	if !revalidated {
		t.Errorf("second read never sent If-None-Match; headers seen: %q", sentIfNoneMatch)
	}
}

// TestNoCacheStillWorks: a Client built without WithCache must behave exactly
// as before — unconditional requests, no nil dereference.
func TestNoCacheStillWorks(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("If-None-Match") != "" {
			t.Errorf("cacheless client sent If-None-Match %q", r.Header.Get("If-None-Match"))
		}
		if strings.Contains(r.URL.Path, "/commits/") {
			return jsonResponse(`{"state":"success"}`, ""), nil
		}
		return jsonResponse(prListBody, `W/"etag-1"`), nil
	})
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: transport}))

	if _, err := c.OpenPRs(context.Background()); err != nil {
		t.Fatalf("OpenPRs without a cache: %v", err)
	}
}

// TestRateLimitIsRecordedAndShortCircuitsLaterRequests: once GitHub says the
// budget is gone, every later request must fail fast without spending another
// one — including from a different Client sharing the cache.
func TestRateLimitIsRecordedAndShortCircuitsLaterRequests(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	reset := now.Add(30 * time.Minute)

	var calls int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		h := make(http.Header)
		h.Set("X-RateLimit-Remaining", "0")
		h.Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"message":"rate limit"}`)), Header: h}, nil
	})

	cache := NewCache()
	client := func(at time.Time) *Client {
		return New("acme", "widgets", "tok",
			WithHTTPClient(&http.Client{Transport: transport}),
			WithCache(cache), withNow(func() time.Time { return at }))
	}

	_, err := client(now).OpenPRs(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("OpenPRs error = %v, want it to wrap ErrRateLimited", err)
	}
	// The downgrade path in internal/api keys off ErrGitHubUnavailable, so a
	// rate limit must still satisfy it — otherwise the board stops rendering
	// instead of degrading.
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Errorf("ErrRateLimited must also wrap ErrGitHubUnavailable, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("first OpenPRs made %d requests, want 1", calls)
	}

	// A fresh client, still inside the window: must not spend a request.
	if _, err := client(now.Add(time.Minute)).OpenPRs(context.Background()); !errors.Is(err, ErrRateLimited) {
		t.Errorf("OpenPRs inside the rate-limit window = %v, want ErrRateLimited", err)
	}
	if calls != 1 {
		t.Errorf("requests made while rate limited = %d, want the limit to short-circuit them", calls)
	}

	// Past the reset, requests resume.
	if _, err := client(reset.Add(time.Second)).OpenPRs(context.Background()); !errors.Is(err, ErrRateLimited) {
		t.Errorf("OpenPRs past reset = %v, want it to try again (and hit the still-limiting stub)", err)
	}
	if calls != 2 {
		t.Errorf("requests after reset = %d, want a second attempt", calls)
	}
}

// TestPlainForbiddenIsNotTreatedAsRateLimit: a 403 without rate-limit headers
// is an authorization failure. Backing off for an hour would hide a bad token
// behind a symptom that looks like throttling.
func TestPlainForbiddenIsNotTreatedAsRateLimit(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"message":"bad credentials"}`)), Header: make(http.Header)}, nil
	})
	cache := NewCache()
	c := New("acme", "widgets", "tok", WithHTTPClient(&http.Client{Transport: transport}), WithCache(cache))

	_, err := c.OpenPRs(context.Background())
	if !errors.Is(err, ErrGitHubUnavailable) {
		t.Fatalf("OpenPRs error = %v, want ErrGitHubUnavailable", err)
	}
	if errors.Is(err, ErrRateLimited) {
		t.Errorf("a plain 403 was misread as a rate limit: %v", err)
	}
	if _, limited := cache.rateLimitedUntilTime(time.Now()); limited {
		t.Error("a plain 403 must not put the cache into rate-limited backoff")
	}
}

// TestRetryAfterIsHonored covers GitHub's secondary (abuse) limit, which
// signals with Retry-After rather than the X-RateLimit-* family.
func TestRetryAfterIsHonored(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("Retry-After", "60")
		return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(`{}`)), Header: h}, nil
	})
	cache := NewCache()
	c := New("acme", "widgets", "tok",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithCache(cache), withNow(func() time.Time { return now }))

	if _, err := c.OpenPRs(context.Background()); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("OpenPRs error = %v, want ErrRateLimited", err)
	}
	reset, limited := cache.rateLimitedUntilTime(now)
	if !limited {
		t.Fatal("Retry-After did not put the cache into backoff")
	}
	if want := now.Add(60 * time.Second); !reset.Equal(want) {
		t.Errorf("backoff until %s, want %s", reset, want)
	}
}

// TestCacheIsBounded: the CI endpoint is keyed by commit SHA, so entries
// churn forever as branches advance. An unbounded map would be a slow leak.
func TestCacheIsBounded(t *testing.T) {
	t.Parallel()
	cache := NewCache()
	for i := range maxCacheEntries * 2 {
		cache.store("https://api.github.com/x/"+strconv.Itoa(i), `W/"e"`, []byte("{}"))
	}
	cache.mu.Lock()
	n := len(cache.entries)
	cache.mu.Unlock()
	if n > maxCacheEntries {
		t.Errorf("cache holds %d entries, want at most %d", n, maxCacheEntries)
	}
}

// TestCacheIsConcurrencySafe: one Cache is shared by every Client in the
// process, and the refresher polls projects while HTTP handlers read boards.
func TestCacheIsConcurrencySafe(t *testing.T) {
	t.Parallel()
	cache := NewCache()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			url := "https://api.github.com/x/" + strconv.Itoa(i%7)
			cache.store(url, `W/"e"`, []byte("{}"))
			cache.lookup(url)
			cache.rateLimitedUntilTime(time.Now())
			cache.setRateLimitedUntil(time.Time{})
		}()
	}
	wg.Wait()
}
