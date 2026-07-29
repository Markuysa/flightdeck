package github

import (
	"sync"
	"time"
)

// maxCacheEntries bounds the conditional-request cache. The CI endpoint is
// keyed by commit SHA, so entries churn as branches advance and an unbounded
// map would grow for the life of the process. Eviction is arbitrary (Go map
// order), not LRU: a wrongly evicted entry costs exactly one uncached
// request, which is not worth a heap of bookkeeping to avoid.
const maxCacheEntries = 1024

// Cache holds the cross-request state a single Client cannot: ETags for
// conditional requests, and how long GitHub has told us to stop asking.
//
// It exists because Clients are cheap and short-lived — internal/api builds
// a fresh one per board read, per project — so anything stored on a Client
// dies before it can be reused. The composition root builds one Cache and
// hands it to every Client through WithCache; that is what makes the ETags
// below actually save anything.
//
// Why this does not violate ADR-001's "derive, never store": nothing here is
// a ticket status. An ETag is a claim about an HTTP resource's version, and a
// 304 means GitHub itself asserts the bytes are unchanged — the board is
// still derived fresh from those bytes on every read. What is skipped is the
// re-transfer, never the derivation.
//
// All methods are safe for concurrent use and safe to call on a nil *Cache,
// which behaves as a cache that never hits — so a Client built without
// WithCache simply makes unconditional requests.
type Cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	// rateLimitedUntil is when GitHub's rate limit is expected to reset.
	// Zero means "not currently limited".
	rateLimitedUntil time.Time
}

// cacheEntry is one URL's last successful response: the ETag to revalidate
// with, and the body to reuse when GitHub answers 304.
type cacheEntry struct {
	etag string
	body []byte
}

// NewCache returns a Cache ready for use. Build one per process and share
// it across every Client — see the type's doc comment for why.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]cacheEntry)}
}

// lookup returns the cached entry for url, if any.
func (c *Cache) lookup(url string) (cacheEntry, bool) {
	if c == nil {
		return cacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[url]
	return e, ok
}

// store records url's ETag and body, evicting arbitrary entries first when
// the cache is at capacity.
func (c *Cache) store(url, etag string, body []byte) {
	if c == nil || etag == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]cacheEntry)
	}
	for len(c.entries) >= maxCacheEntries {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[url] = cacheEntry{etag: etag, body: body}
}

// rateLimitedUntilTime reports whether GitHub's rate limit is currently
// believed to be exhausted, and when it resets.
func (c *Cache) rateLimitedUntilTime(now time.Time) (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rateLimitedUntil.IsZero() || !now.Before(c.rateLimitedUntil) {
		return time.Time{}, false
	}
	return c.rateLimitedUntil, true
}

// setRateLimitedUntil records that GitHub refused us until reset. Every
// Client sharing this Cache then fails fast until then instead of spending
// more requests learning the same thing.
func (c *Cache) setRateLimitedUntil(reset time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rateLimitedUntil = reset
}
