package universe

// IsNifty50 reports membership using SYMBOL.NS format per docs/rules.md.
func IsNifty50(symbol string) bool {
	_, ok := nifty50[symbol]
	return ok
}

var nifty50 = map[string]struct{}{
	"ADANIENT.NS": {}, "ADANIPORTS.NS": {}, "APOLLOHOSP.NS": {}, "ASIANPAINT.NS": {}, "AXISBANK.NS": {},
	"BAJAJ-AUTO.NS": {}, "BAJFINANCE.NS": {}, "BAJAJFINSV.NS": {}, "BPCL.NS": {}, "BHARTIARTL.NS": {},
	"BRITANNIA.NS": {}, "CIPLA.NS": {}, "COALINDIA.NS": {}, "DIVISLAB.NS": {}, "DRREDDY.NS": {},
	"EICHERMOT.NS": {}, "GRASIM.NS": {}, "HCLTECH.NS": {}, "HDFCBANK.NS": {}, "HDFCLIFE.NS": {},
	"HEROMOTOCO.NS": {}, "HINDALCO.NS": {}, "HINDUNILVR.NS": {}, "ICICIBANK.NS": {}, "ITC.NS": {},
	"INDUSINDBK.NS": {}, "INFY.NS": {}, "JSWSTEEL.NS": {}, "KOTAKBANK.NS": {}, "LT.NS": {},
	"LTIM.NS": {}, "M&M.NS": {}, "MARUTI.NS": {}, "NESTLEIND.NS": {}, "NTPC.NS": {},
	"ONGC.NS": {}, "POWERGRID.NS": {}, "RELIANCE.NS": {}, "SBILIFE.NS": {}, "SBIN.NS": {},
	"SUNPHARMA.NS": {}, "TATACONSUM.NS": {}, "TATAMOTORS.NS": {}, "TATASTEEL.NS": {}, "TCS.NS": {},
	"TECHM.NS": {}, "TITAN.NS": {}, "ULTRACEMCO.NS": {}, "UPL.NS": {}, "WIPRO.NS": {},
}
