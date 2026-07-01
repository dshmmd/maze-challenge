package domain

// Wallet is a guild's treasury. It tracks total holdings and the portion
// currently reserved against active auction bids.
//
// Invariant: 0 <= Reserved <= Total, therefore Available() >= 0.
//
// The Wallet methods are pure value-mutators with no I/O. Persistence and
// concurrency control (row locking, transactions) live in the adapter layer;
// this type only enforces the arithmetic rules so they can be unit-tested in
// isolation. A bid "reserves" funds (R: balance incl. reservations checked
// before any commitment); winning "debits"; losing/cancelling "releases".
type Wallet struct {
	GuildID  string
	Total    Gold
	Reserved Gold
	DailyCap Gold // max committed spend per UTC day; 0 means unlimited
}

// Available is the spendable balance: total minus what is already reserved.
func (w Wallet) Available() Gold { return w.Total - w.Reserved }

// WithinDailyCap reports whether committing `amount` keeps the guild's spend for
// the day within its cap, given how much it has already `spentToday` (R14). A
// zero cap means unlimited.
func (w Wallet) WithinDailyCap(spentToday, amount Gold) bool {
	if w.DailyCap <= 0 {
		return true
	}
	return spentToday+amount <= w.DailyCap
}

// Reserve locks `amount` against future spend (placing/raising a bid).
// It fails if the amount is non-positive or exceeds the available balance.
func (w *Wallet) Reserve(amount Gold) error {
	if !amount.IsPositive() {
		return ErrNonPositiveAmount
	}
	if amount > w.Available() {
		return ErrInsufficientFunds
	}
	w.Reserved += amount
	return nil
}

// Release frees a previously reserved amount (losing/cancelling a bid).
// It fails if releasing more than is currently reserved, which would break
// the invariant and signal a double-release bug.
func (w *Wallet) Release(amount Gold) error {
	if !amount.IsPositive() {
		return ErrNonPositiveAmount
	}
	if amount > w.Reserved {
		return ErrReserveTooLarge
	}
	w.Reserved -= amount
	return nil
}

// Settle finalizes a won bid: the reserved amount leaves the wallet for good.
// It both releases the reservation and debits the total in one step so the
// two can never drift apart.
func (w *Wallet) Settle(amount Gold) error {
	if !amount.IsPositive() {
		return ErrNonPositiveAmount
	}
	if amount > w.Reserved {
		return ErrReserveTooLarge
	}
	w.Reserved -= amount
	w.Total -= amount
	return nil
}

// Debit spends directly from available funds without a prior reservation
// (a limit-order purchase, which is immediate). It never touches Reserved.
func (w *Wallet) Debit(amount Gold) error {
	if !amount.IsPositive() {
		return ErrNonPositiveAmount
	}
	if amount > w.Available() {
		return ErrInsufficientFunds
	}
	w.Total -= amount
	return nil
}

// Credit adds funds to the wallet (e.g. proceeds from a completed sale).
func (w *Wallet) Credit(amount Gold) error {
	if !amount.IsPositive() {
		return ErrNonPositiveAmount
	}
	w.Total += amount
	return nil
}
