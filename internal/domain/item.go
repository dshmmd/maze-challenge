package domain

import "time"

// Rarity classifies an item and decides how it is sold.
type Rarity string

const (
	// Common — plentiful, reproducible. Sold via limit order.
	Common Rarity = "common"
	// Rare — limited stock, reproducible but slowly. Sold via limit order.
	Rare Rarity = "rare"
	// Legendary — exactly one copy in existence, never reproduced. Sold only
	// via auction (see R3/R5), never by limit order.
	Legendary Rarity = "legendary"
)

// Valid reports whether r is a known rarity.
func (r Rarity) Valid() bool {
	switch r {
	case Common, Rare, Legendary:
		return true
	default:
		return false
	}
}

// AuctionOnly reports whether items of this rarity can only be sold by auction.
func (r Rarity) AuctionOnly() bool { return r == Legendary }

// ItemStatus is the lifecycle state of an item/listing.
type ItemStatus string

const (
	// ItemAvailable — listed and purchasable (or auctionable, for Legendary).
	ItemAvailable ItemStatus = "available"
	// ItemSoldOut — a limit-order listing whose stock reached zero.
	ItemSoldOut ItemStatus = "sold_out"
	// ItemInAuction — a Legendary item with an active auction (Phase 2).
	ItemInAuction ItemStatus = "in_auction"
)

// Item is something a guild lists for sale. For Common/Rare it is a limit-order
// listing carrying a fixed unit Price and a remaining Quantity (D9). For
// Legendary, Quantity is always 1 and Price is unused (it is auctioned).
type Item struct {
	ID            string
	Name          string
	Rarity        Rarity
	SellerGuildID string
	Price         Gold // unit price for limit orders; 0 for Legendary
	Quantity      int  // remaining units; 1 for Legendary
	Status        ItemStatus
	CreatedAt     time.Time
}

// NewItem validates inputs and constructs an Item in the Available state.
// Legendary items ignore price/quantity and are fixed to a single unit.
func NewItem(id, name string, rarity Rarity, seller string, price Gold, quantity int, now time.Time) (*Item, error) {
	if !rarity.Valid() {
		return nil, ErrInvalidRarity
	}
	item := &Item{
		ID:            id,
		Name:          name,
		Rarity:        rarity,
		SellerGuildID: seller,
		Status:        ItemAvailable,
		CreatedAt:     now,
	}
	if rarity.AuctionOnly() {
		item.Price = 0
		item.Quantity = 1
		return item, nil
	}
	if !price.IsPositive() {
		return nil, ErrNonPositiveAmount
	}
	if quantity < 1 {
		return nil, ErrInvalidQuantity
	}
	item.Price = price
	item.Quantity = quantity
	return item, nil
}

// SellTo validates and applies a limit-order purchase of `qty` units by
// `buyer`, enforcing the rules: not the seller's own item (R10), not a
// Legendary (those are auction-only), the listing is available, and there is
// enough stock. It mutates Quantity/Status and returns the total cost. Funds
// movement is the caller's responsibility (done atomically in the same tx).
func (i *Item) SellTo(buyer string, qty int) (Gold, error) {
	if qty < 1 {
		return 0, ErrInvalidQuantity
	}
	if i.Rarity.AuctionOnly() {
		return 0, ErrLegendaryNotLimit
	}
	if i.SellerGuildID == buyer {
		return 0, ErrCannotBuyOwnItem
	}
	if i.Status != ItemAvailable {
		return 0, ErrItemNotAvailable
	}
	if qty > i.Quantity {
		return 0, ErrInsufficientStock
	}
	cost := i.Price * Gold(qty)
	i.Quantity -= qty
	if i.Quantity == 0 {
		i.Status = ItemSoldOut
	}
	return cost, nil
}
