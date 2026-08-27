package validation

import "testing"

func TestOrderTypes(t *testing.T) {
	t.Parallel()
	if err := Order("TCS.NS", "market", "buy", 1, nil); err != nil {
		t.Fatal(err)
	}
	px := 100.0
	if err := Order("TCS.NS", "limit", "buy", 1, &px); err != nil {
		t.Fatal(err)
	}
	if err := Order("TCS.NS", "limit", "buy", 1, nil); err != ErrLimitPrice {
		t.Fatalf("expected limit price error, got %v", err)
	}
	if err := Order("TCS.NS", "stop", "sell", 1, &px); err != nil {
		t.Fatal(err)
	}
	if err := MarketOrder("TCS.NS", "market", "buy", 1); err != nil {
		t.Fatal(err)
	}
	if err := MarketOrder("TCS.NS", "limit", "buy", 1); err != ErrLimitPrice {
		t.Fatalf("market helper with limit and no price: %v", err)
	}
}
