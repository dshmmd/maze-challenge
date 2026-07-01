package domain

import "errors"

// Domain-level sentinel errors. Adapters (HTTP) map these to status codes;
// the domain never knows about transport. Keeping them here means the
// business rules in ROADMAP's R-ledger have a single, greppable home.
var (
	// Wallet / funds
	ErrInsufficientFunds = errors.New("insufficient available funds")
	ErrNonPositiveAmount = errors.New("amount must be positive")
	ErrReserveTooLarge   = errors.New("cannot release more than is reserved")

	// Items / listings
	ErrItemNotFound       = errors.New("item not found")
	ErrItemNotAvailable   = errors.New("item is not available for sale")
	ErrCannotBuyOwnItem   = errors.New("guild cannot buy its own item")
	ErrNotItemOwner       = errors.New("only the item owner may perform this action")
	ErrInvalidRarity      = errors.New("invalid item rarity")
	ErrInvalidQuantity    = errors.New("quantity must be positive")
	ErrInsufficientStock  = errors.New("not enough stock to fulfil purchase")
	ErrLegendaryNotLimit  = errors.New("legendary items are sold by auction, not limit order")
	ErrNonLegendaryNoBids = errors.New("only legendary items can be auctioned")

	// Guilds / wallets
	ErrWalletNotFound = errors.New("wallet not found")
	ErrGuildNotFound  = errors.New("guild not found")

	// Auctions / bids
	ErrAuctionNotFound    = errors.New("auction not found")
	ErrAuctionClosed      = errors.New("auction is not active")
	ErrAuctionExists      = errors.New("item already has an active auction")
	ErrNotLegendary       = errors.New("only legendary items can be auctioned")
	ErrBidTooLow          = errors.New("bid must exceed current highest by the minimum increment")
	ErrCannotBidOwnItem   = errors.New("guild cannot bid on its own item")
	ErrCannotCancelLeader = errors.New("highest bidder cannot cancel their bid")
	ErrBidNotFound        = errors.New("bid not found")

	// Guild limits
	ErrDailyCapExceeded = errors.New("guild daily purchase cap exceeded")
)
