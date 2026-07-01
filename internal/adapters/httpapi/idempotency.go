package httpapi

import (
	"bytes"
	"net/http"
	"sync"
)

// idempotencyStore caches the first response for a given Idempotency-Key so a
// replayed mutating request returns the original result without re-applying it
// (R16, D14). This is an in-process cache: simple and dependency-free, but not
// durable across restarts — an acceptable off-trade documented in the ADR.
//
// Natural invariants (stock, single-active-auction, balance checks) already
// prevent double-effect at the data layer; this adds belt-and-suspenders for
// client retries.
type idempotencyStore struct {
	mu      sync.Mutex
	entries map[string]idempotentResponse
}

type idempotentResponse struct {
	status int
	body   []byte
}

func newIdempotencyStore() *idempotencyStore {
	return &idempotencyStore{entries: map[string]idempotentResponse{}}
}

// capture is a ResponseWriter that records what was written so it can be cached.
type capture struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (c *capture) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *capture) Write(b []byte) (int, error) {
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

// middleware replays a cached response for a seen key, or records the first one.
// Keys are scoped per acting guild + method + path so they cannot collide across
// guilds or endpoints. Requests without the header pass through unchanged.
func (s *idempotencyStore) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		scoped := guildID(r) + "|" + r.Method + "|" + r.URL.Path + "|" + key

		s.mu.Lock()
		if prev, ok := s.entries[scoped]; ok {
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(prev.status)
			_, _ = w.Write(prev.body)
			return
		}
		s.mu.Unlock()

		cw := &capture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(cw, r)

		// Only cache successful, applied mutations (2xx).
		if cw.status >= 200 && cw.status < 300 {
			s.mu.Lock()
			s.entries[scoped] = idempotentResponse{status: cw.status, body: cw.buf.Bytes()}
			s.mu.Unlock()
		}
	})
}
