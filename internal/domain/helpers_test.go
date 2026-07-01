package domain

import "time"

// mockTime is a fixed instant for deterministic domain tests.
func mockTime() time.Time {
	return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
}
