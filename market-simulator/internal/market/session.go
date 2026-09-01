package market

import "time"

// IST is Asia/Kolkata (UTC+5:30), used for NSE session dates.
var IST = time.FixedZone("IST", 5*3600+30*60)

const Interval1d = "1d"
const Interval60m = "60m"

// NormalizeInterval maps config / query values to 1d or 60m.
func NormalizeInterval(s string) string {
	switch s {
	case Interval60m, "60min", "1h", "60":
		return Interval60m
	default:
		return Interval1d
	}
}

// SessionDate is the NSE calendar date for a timestamp (IST).
func SessionDate(ts time.Time) time.Time {
	if ts.IsZero() {
		return ts
	}
	t := ts.In(IST)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, IST)
}

// SessionCalendarUTC is SessionDate stored as a UTC midnight date (Postgres DATE).
func SessionCalendarUTC(ts time.Time) time.Time {
	d := SessionDate(ts)
	if d.IsZero() {
		return d
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// SameSession is true when two timestamps fall on the same IST trading date.
func SameSession(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	da, db := SessionDate(a), SessionDate(b)
	return da.Year() == db.Year() && da.Month() == db.Month() && da.Day() == db.Day()
}

// ISTDayRangeUTC returns [start, end) UTC covering the IST calendar day of `day`.
func ISTDayRangeUTC(day time.Time) (start, endExclusive time.Time) {
	d := SessionDate(day)
	if day.Location() == time.UTC || day.Location() == nil {
		// date-only values (UTC midnight) mean that civil date in IST
		y, m, dd := day.UTC().Date()
		d = time.Date(y, m, dd, 0, 0, 0, 0, IST)
	}
	start = d.UTC()
	endExclusive = d.Add(24 * time.Hour).UTC()
	return start, endExclusive
}

// InclusiveDateRangeUTC returns UTC bounds covering IST sessions from startDate through endDate inclusive.
func InclusiveDateRangeUTC(startDate, endDate time.Time) (start, endExclusive time.Time) {
	start, _ = ISTDayRangeUTC(startDate)
	_, endExclusive = ISTDayRangeUTC(endDate)
	return start, endExclusive
}
