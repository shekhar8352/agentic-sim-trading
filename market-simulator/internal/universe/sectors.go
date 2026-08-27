package universe

// SectorOf returns the NSE sector for a Nifty 50 ticker (fallback when stocks.sector is unset).
func SectorOf(symbol string) string {
	if s, ok := sectors[symbol]; ok {
		return s
	}
	return "Unknown"
}

var sectors = map[string]string{
	"ADANIENT.NS":   "Metals",
	"ADANIPORTS.NS": "Infrastructure",
	"APOLLOHOSP.NS": "Healthcare",
	"ASIANPAINT.NS": "Consumer",
	"AXISBANK.NS":   "Financials",
	"BAJAJ-AUTO.NS": "Auto",
	"BAJFINANCE.NS": "Financials",
	"BAJAJFINSV.NS": "Financials",
	"BPCL.NS":       "Energy",
	"BHARTIARTL.NS": "Telecom",
	"BRITANNIA.NS":  "FMCG",
	"CIPLA.NS":      "Healthcare",
	"COALINDIA.NS":  "Energy",
	"DIVISLAB.NS":   "Healthcare",
	"DRREDDY.NS":    "Healthcare",
	"EICHERMOT.NS":  "Auto",
	"GRASIM.NS":     "Materials",
	"HCLTECH.NS":    "IT",
	"HDFCBANK.NS":   "Financials",
	"HDFCLIFE.NS":   "Financials",
	"HEROMOTOCO.NS": "Auto",
	"HINDALCO.NS":   "Metals",
	"HINDUNILVR.NS": "FMCG",
	"ICICIBANK.NS":  "Financials",
	"ITC.NS":        "FMCG",
	"INDUSINDBK.NS": "Financials",
	"INFY.NS":       "IT",
	"JSWSTEEL.NS":   "Metals",
	"KOTAKBANK.NS":  "Financials",
	"LT.NS":         "Industrials",
	"LTIM.NS":       "IT",
	"M&M.NS":        "Auto",
	"MARUTI.NS":     "Auto",
	"NESTLEIND.NS":  "FMCG",
	"NTPC.NS":       "Energy",
	"ONGC.NS":       "Energy",
	"POWERGRID.NS":  "Energy",
	"RELIANCE.NS":   "Energy",
	"SBILIFE.NS":    "Financials",
	"SBIN.NS":       "Financials",
	"SUNPHARMA.NS":  "Healthcare",
	"TATACONSUM.NS": "FMCG",
	"TATAMOTORS.NS": "Auto",
	"TATASTEEL.NS":  "Metals",
	"TCS.NS":        "IT",
	"TECHM.NS":      "IT",
	"TITAN.NS":      "Consumer",
	"ULTRACEMCO.NS": "Materials",
	"UPL.NS":        "Materials",
	"WIPRO.NS":      "IT",
}
