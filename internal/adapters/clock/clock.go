// Package clock provides Clock implementations for production and tests.
package clock

import (
	"sync"
	"time"
)

// Real is the production clock backed by the wall clock.
type Real struct{}

// Now returns the current wall-clock time.
func (Real) Now() time.Time { return time.Now() }

// Fake is a controllable clock for tests. Time only moves when Advance or Set
// is called, which makes auction-window and anti-snipe logic deterministic (D4).
// It is safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a Fake clock fixed at the given instant.
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// Now returns the fake clock's current instant.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the fake clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// Set pins the fake clock to a specific instant.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}
