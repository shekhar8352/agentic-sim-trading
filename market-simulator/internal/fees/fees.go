package fees

// Rules docs/rules.md §8 — components applied to trade value V.

// Breakdown is the per-component fee audit stored on the order row.
type Breakdown struct {
	Brokerage float64 `json:"brokerage"`
	STT       float64 `json:"stt"`
	GST       float64 `json:"gst"`
	Exchange  float64 `json:"exchange"`
	SEBI      float64 `json:"sebi"`
	Stamp     float64 `json:"stamp"`
	Total     float64 `json:"total"`
}

func (b Breakdown) TotalDebit(tradeValue float64) float64 {
	return tradeValue + b.Total
}

func (b Breakdown) TotalCredit(tradeValue float64) float64 {
	return tradeValue - b.Total
}

// BuyFees returns buy-side components (no STT; stamp duty applies).
func BuyFees(tradeValue float64) Breakdown {
	brokerage := 0.001 * tradeValue
	gst := 0.18 * brokerage
	exchange := 0.0000345 * tradeValue
	sebi := 0.000001 * tradeValue
	stamp := 0.00015 * tradeValue
	return Breakdown{
		Brokerage: brokerage,
		GST:       gst,
		Exchange:  exchange,
		SEBI:      sebi,
		Stamp:     stamp,
		Total:     brokerage + gst + exchange + sebi + stamp,
	}
}

// SellFees returns sell-side components (STT applies; no stamp).
func SellFees(tradeValue float64) Breakdown {
	brokerage := 0.001 * tradeValue
	gst := 0.18 * brokerage
	exchange := 0.0000345 * tradeValue
	sebi := 0.000001 * tradeValue
	stt := 0.001 * tradeValue
	return Breakdown{
		Brokerage: brokerage,
		GST:       gst,
		Exchange:  exchange,
		SEBI:      sebi,
		STT:       stt,
		Total:     brokerage + gst + exchange + sebi + stt,
	}
}

// BuyDebit returns cash leaving the account (V + all buy-side fees).
func BuyDebit(tradeValue float64) (totalDebit float64, feesTotal float64) {
	b := BuyFees(tradeValue)
	return b.TotalDebit(tradeValue), b.Total
}

// SellCredit returns cash credited after sell-side fees (STT on sells).
func SellCredit(tradeValue float64) (totalCredit float64, feesTotal float64) {
	b := SellFees(tradeValue)
	return b.TotalCredit(tradeValue), b.Total
}
