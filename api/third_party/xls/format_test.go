package xls

import "testing"

func TestIsDateFormat(t *testing.T) {
	cases := map[string]bool{
		`[$-421]dd mmmm yyyy`:       true,
		`[h]:mm`:                    true,
		`0.00E+00`:                  false,
		`"Rp"#,##0;[Red]-"Rp"#,##0`: false,
		`@`:                         false,
		`#,##0_);[Red](#,##0)`:      false,
		`d/m/yyyy`:                  true,
		`"Rp" #,##0`:                false,
		`0000000000`:                false,
		`General`:                   false,
		`\d0`:                       false,
	}
	for f, want := range cases {
		if got := isDateFormat(f); got != want {
			t.Errorf("isDateFormat(%q)=%v want %v", f, got, want)
		}
	}
}
