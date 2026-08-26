package fees

import "testing"

func TestBuyFeesExampleTCS(t *testing.T) {
	t.Parallel()
	// docs/rules.md §8 example: 10 × ₹3,500 = ₹35,000
	b := BuyFees(35_000)
	if abs(b.Brokerage-35) > 0.001 {
		t.Fatalf("brokerage: got %v", b.Brokerage)
	}
	if abs(b.GST-6.3) > 0.001 {
		t.Fatalf("gst: got %v", b.GST)
	}
	if abs(b.Stamp-5.25) > 0.001 {
		t.Fatalf("stamp: got %v", b.Stamp)
	}
	debit, total := BuyDebit(35_000)
	if abs(total-b.Total) > 1e-9 {
		t.Fatalf("BuyDebit total mismatch")
	}
	if abs(debit-(35_000+total)) > 1e-9 {
		t.Fatalf("debit: got %v", debit)
	}
}

func TestSellFeesIncludeSTT(t *testing.T) {
	t.Parallel()
	s := SellFees(10_000)
	if s.STT != 10 {
		t.Fatalf("stt: got %v", s.STT)
	}
	if s.Stamp != 0 {
		t.Fatalf("sell should not have stamp: %v", s.Stamp)
	}
	credit, total := SellCredit(10_000)
	if abs(credit-(10_000-total)) > 1e-9 {
		t.Fatalf("credit: got %v", credit)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
