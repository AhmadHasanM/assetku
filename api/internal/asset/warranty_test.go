package asset

import (
	"testing"

	"cloud.google.com/go/civil"
)

func date(s string) *civil.Date {
	d, err := civil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return &d
}

func months(n int) *int { return &n }

func TestWarrantyExpires(t *testing.T) {
	cases := []struct {
		purchase *civil.Date
		months   *int
		want     *civil.Date
	}{
		{date("2026-01-15"), months(12), date("2027-01-15")},
		{date("2026-01-31"), months(1), date("2026-02-28")},
		{date("2028-01-31"), months(1), date("2028-02-29")},
		{date("2026-03-31"), months(1), date("2026-04-30")},
		{date("2026-05-10"), months(0), date("2026-05-10")},
		{date("2026-01-15"), nil, nil},
		{nil, months(12), nil},
	}
	for _, c := range cases {
		got := WarrantyExpires(c.purchase, c.months)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("WarrantyExpires(%v, %v) = %v, want %v", c.purchase, c.months, got, c.want)
		}
	}
}

func TestAddMonthsClampedNegative(t *testing.T) {
	if got := AddMonthsClamped(*date("2026-03-31"), -1); got != *date("2026-02-28") {
		t.Fatalf("got %v", got)
	}
	if got := AddMonthsClamped(*date("2026-12-31"), 2); got != *date("2027-02-28") {
		t.Fatalf("got %v", got)
	}
}
