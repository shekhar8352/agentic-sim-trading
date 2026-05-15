package fees

// Rules docs/rules.md §8 — components applied to trade value V.

// BuyDebit returns cash leaving the account (V + all buy-side fees).
func BuyDebit(tradeValue float64) (totalDebit float64, feesTotal float64) {
	brokerage := 0.001 * tradeValue
	gst := 0.18 * brokerage
	exchange := 0.0000345 * tradeValue
	sebi := 0.000001 * tradeValue
	stamp := 0.00015 * tradeValue
	feesTotal = brokerage + gst + exchange + sebi + stamp
	totalDebit = tradeValue + feesTotal
	return totalDebit, feesTotal
}

// SellCredit returns cash credited after sell-side fees (STT on sells).
func SellCredit(tradeValue float64) (totalCredit float64, feesTotal float64) {
	brokerage := 0.001 * tradeValue
	gst := 0.18 * brokerage
	exchange := 0.0000345 * tradeValue
	sebi := 0.000001 * tradeValue
	stt := 0.001 * tradeValue
	feesTotal = brokerage + gst + exchange + sebi + stt
	totalCredit = tradeValue - feesTotal
	return totalCredit, feesTotal
}
