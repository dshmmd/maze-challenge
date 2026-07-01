package domain

// Gold is the market's single currency, stored as a whole-unit integer.
//
// We deliberately avoid floating point: money must be exact, and every
// reserve/release/debit has to net to zero. int64 of whole gold gives us
// ~9.2e18 of headroom, far beyond any guild's treasury.
type Gold int64

// IsPositive reports whether the amount is strictly greater than zero.
// Prices and bids must always be positive — a zero or negative value is a
// signal of bad input (or a misbehaving Price Oracle).
func (g Gold) IsPositive() bool { return g > 0 }
