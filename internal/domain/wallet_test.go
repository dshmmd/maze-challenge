package domain

import "testing"

func TestWalletAvailable(t *testing.T) {
	w := Wallet{GuildID: "g1", Total: 100, Reserved: 30}
	if got := w.Available(); got != 70 {
		t.Fatalf("Available() = %d, want 70", got)
	}
}

func TestReserveRejectsOverAvailable(t *testing.T) {
	w := Wallet{GuildID: "g1", Total: 100, Reserved: 80}
	// Only 20 available; reserving 21 must fail and leave the wallet untouched.
	if err := w.Reserve(21); err != ErrInsufficientFunds {
		t.Fatalf("Reserve(21) err = %v, want ErrInsufficientFunds", err)
	}
	if w.Reserved != 80 {
		t.Fatalf("Reserved mutated on failed reserve: %d", w.Reserved)
	}
}

func TestReserveRejectsNonPositive(t *testing.T) {
	w := Wallet{Total: 100}
	for _, amt := range []Gold{0, -5} {
		if err := w.Reserve(amt); err != ErrNonPositiveAmount {
			t.Fatalf("Reserve(%d) err = %v, want ErrNonPositiveAmount", amt, err)
		}
	}
}

func TestReserveReleaseRoundTrip(t *testing.T) {
	w := Wallet{GuildID: "g1", Total: 100}
	if err := w.Reserve(40); err != nil {
		t.Fatalf("Reserve(40): %v", err)
	}
	if w.Available() != 60 {
		t.Fatalf("Available after reserve = %d, want 60", w.Available())
	}
	if err := w.Release(40); err != nil {
		t.Fatalf("Release(40): %v", err)
	}
	if w.Reserved != 0 || w.Total != 100 {
		t.Fatalf("after round trip: total=%d reserved=%d, want 100/0", w.Total, w.Reserved)
	}
}

func TestReleaseCannotExceedReserved(t *testing.T) {
	w := Wallet{Total: 100, Reserved: 10}
	if err := w.Release(11); err != ErrReserveTooLarge {
		t.Fatalf("Release(11) err = %v, want ErrReserveTooLarge", err)
	}
}

func TestSettleReleasesAndDebits(t *testing.T) {
	w := Wallet{GuildID: "g1", Total: 100, Reserved: 40}
	if err := w.Settle(40); err != nil {
		t.Fatalf("Settle(40): %v", err)
	}
	if w.Total != 60 || w.Reserved != 0 {
		t.Fatalf("after settle: total=%d reserved=%d, want 60/0", w.Total, w.Reserved)
	}
}

func TestDebitIgnoresReserved(t *testing.T) {
	// Available is 50 (100 total - 50 reserved). A 60 debit must fail.
	w := Wallet{Total: 100, Reserved: 50}
	if err := w.Debit(60); err != ErrInsufficientFunds {
		t.Fatalf("Debit(60) err = %v, want ErrInsufficientFunds", err)
	}
	if err := w.Debit(50); err != nil {
		t.Fatalf("Debit(50): %v", err)
	}
	if w.Total != 50 || w.Reserved != 50 {
		t.Fatalf("after debit: total=%d reserved=%d, want 50/50", w.Total, w.Reserved)
	}
}
