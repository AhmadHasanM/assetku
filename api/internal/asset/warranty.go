package asset

import (
	"time"
	_ "time/tzdata" // Asia/Jakarta must resolve even without system zoneinfo

	"cloud.google.com/go/civil"
)

// Jakarta is the zone in which "today" is evaluated for warranty, BAST and reminders.
var Jakarta = mustLoad("Asia/Jakarta")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Today returns the current date in Asia/Jakarta.
func Today() civil.Date { return civil.DateOf(time.Now().In(Jakarta)) }

// AddMonthsClamped adds n months (n may be negative), clamping the day to the
// last day of the target month: 31 Jan + 1 = 28/29 Feb.
func AddMonthsClamped(d civil.Date, n int) civil.Date {
	first := time.Date(d.Year, d.Month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
	last := first.AddDate(0, 1, -1).Day()
	day := min(d.Day, last)
	return civil.Date{Year: first.Year(), Month: first.Month(), Day: day}
}

// WarrantyExpires is purchase + months (clamped), or nil when either is nil.
func WarrantyExpires(purchase *civil.Date, months *int) *civil.Date {
	if purchase == nil || months == nil {
		return nil
	}
	d := AddMonthsClamped(*purchase, *months)
	return &d
}
