package domain

import "time"

// LedgerEntryType names a kind of gold movement. Every commitment is recorded
// so a guild's balance is fully traceable (R9): you can replay the ledger and
// arrive at the same total/reserved.
type LedgerEntryType string

const (
	LedgerDebit   LedgerEntryType = "debit"   // funds left the wallet (purchase)
	LedgerCredit  LedgerEntryType = "credit"  // funds entered the wallet (sale proceeds)
	LedgerReserve LedgerEntryType = "reserve" // funds locked for a bid
	LedgerRelease LedgerEntryType = "release" // bid reservation freed
	LedgerSettle  LedgerEntryType = "settle"  // won bid finalized
)

// LedgerEntry is one immutable, traceable record of a gold movement.
type LedgerEntry struct {
	ID      string
	GuildID string
	Type    LedgerEntryType
	Amount  Gold
	ItemID  string // related item, if any
	Memo    string
	At      time.Time
}
