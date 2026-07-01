package domain

import "testing"

func newRareItem(t *testing.T, seller string, price Gold, qty int) *Item {
	t.Helper()
	it, err := NewItem("i1", "Healing Potion", Rare, seller, price, qty, mockTime())
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	return it
}

func TestNewItemRejectsBadInputs(t *testing.T) {
	if _, err := NewItem("i", "x", Rarity("mythic"), "g1", 10, 1, mockTime()); err != ErrInvalidRarity {
		t.Fatalf("bad rarity err = %v, want ErrInvalidRarity", err)
	}
	if _, err := NewItem("i", "x", Rare, "g1", 0, 1, mockTime()); err != ErrNonPositiveAmount {
		t.Fatalf("zero price err = %v, want ErrNonPositiveAmount", err)
	}
	if _, err := NewItem("i", "x", Rare, "g1", 10, 0, mockTime()); err != ErrInvalidQuantity {
		t.Fatalf("zero qty err = %v, want ErrInvalidQuantity", err)
	}
}

func TestNewLegendaryIsSingleUnitNoPrice(t *testing.T) {
	it, err := NewItem("i", "Soul Reaver", Legendary, "g1", 999, 7, mockTime())
	if err != nil {
		t.Fatalf("NewItem legendary: %v", err)
	}
	if it.Quantity != 1 || it.Price != 0 {
		t.Fatalf("legendary qty=%d price=%d, want 1/0", it.Quantity, it.Price)
	}
}

func TestSellToHappyPathAndSellOut(t *testing.T) {
	it := newRareItem(t, "seller", 50, 2)
	cost, err := it.SellTo("buyer", 2)
	if err != nil {
		t.Fatalf("SellTo: %v", err)
	}
	if cost != 100 {
		t.Fatalf("cost = %d, want 100", cost)
	}
	if it.Quantity != 0 || it.Status != ItemSoldOut {
		t.Fatalf("after full buy: qty=%d status=%s, want 0/sold_out", it.Quantity, it.Status)
	}
}

func TestSellToRejectsOwnItem(t *testing.T) {
	it := newRareItem(t, "g1", 50, 1)
	if _, err := it.SellTo("g1", 1); err != ErrCannotBuyOwnItem {
		t.Fatalf("SellTo own item err = %v, want ErrCannotBuyOwnItem", err)
	}
}

func TestSellToRejectsOverStock(t *testing.T) {
	it := newRareItem(t, "seller", 50, 1)
	if _, err := it.SellTo("buyer", 2); err != ErrInsufficientStock {
		t.Fatalf("SellTo over stock err = %v, want ErrInsufficientStock", err)
	}
}

func TestSellToRejectsSoldOut(t *testing.T) {
	it := newRareItem(t, "seller", 50, 1)
	if _, err := it.SellTo("buyer", 1); err != nil {
		t.Fatalf("first buy: %v", err)
	}
	// Second buyer hits the now sold-out listing.
	if _, err := it.SellTo("buyer2", 1); err != ErrItemNotAvailable {
		t.Fatalf("SellTo sold out err = %v, want ErrItemNotAvailable", err)
	}
}

func TestSellToRejectsLegendary(t *testing.T) {
	it, _ := NewItem("i", "Eye of the Dragon", Legendary, "seller", 0, 1, mockTime())
	if _, err := it.SellTo("buyer", 1); err != ErrLegendaryNotLimit {
		t.Fatalf("SellTo legendary err = %v, want ErrLegendaryNotLimit", err)
	}
}
