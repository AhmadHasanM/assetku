package importer

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"
)

func TestParseCategory(t *testing.T) {
	cases := []struct {
		in, code string
		ok       bool
	}{
		{"Peralatan IT", "it_equipment", true}, {"laptop", "it_equipment", true}, {" Komputer ", "it_equipment", true},
		{"Kendaraan", "vehicle", true}, {"MOTOR", "vehicle", true},
		{"Fasilitas Gedung", "building_facility", true}, {"building", "building_facility", true},
		{"Furnitur", "furniture", true}, {"mebel", "furniture", true},
		{"Elektronik", "electronics", true}, {"electronic", "electronics", true},
		{"Lain-lain", "other", true}, {"lainnya", "other", true},
		{"Alat Berat", "other", false}, {"", "other", true},
	}
	for _, c := range cases {
		code, ok := ParseCategory(c.in)
		if code != c.code || ok != c.ok {
			t.Errorf("ParseCategory(%q)=%s,%v want %s,%v", c.in, code, ok, c.code, c.ok)
		}
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in, code string
		ok       bool
	}{
		{"", "available", true}, {"Tersedia", "available", true}, {"Ready to Deploy", "available", true},
		{"Digunakan", "in_use", true}, {"deployed", "in_use", true},
		{"Pending", "maintenance", true}, {"Perbaikan", "maintenance", true},
		{"Rusak", "damaged", true}, {"Un-deployable", "damaged", true},
		{"Hilang", "lost", true}, {"Stolen", "lost", true},
		{"Diarsipkan", "archived", true}, {"Arsip", "archived", true},
		{"entah", "available", false},
	}
	for _, c := range cases {
		code, ok := ParseStatus(c.in)
		if code != c.code || ok != c.ok {
			t.Errorf("ParseStatus(%q)=%s,%v want %s,%v", c.in, code, ok, c.code, c.ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	d := func(y int, m time.Month, day int) *civil.Date { return &civil.Date{Year: y, Month: m, Day: day} }
	good := map[string]*civil.Date{
		"":                    d(0, 0, 0),
		"46337":               d(2026, 11, 11),
		"46337.5":             d(2026, 11, 11),
		"45000":               d(2023, 3, 15),
		"11/11/2026":          d(2026, 11, 11),
		"11-11-2026":          d(2026, 11, 11),
		"1/2/2026":            d(2026, 2, 1), // dd/mm, not mm/dd
		"05/03/2026":          d(2026, 3, 5),
		"2026-11-11":          d(2026, 11, 11),
		"2026-11-11 00:00:00": d(2026, 11, 11),
		"11 November 2026":    d(2026, 11, 11),
		"11 Nov 2026":         d(2026, 11, 11),
		"11 Agustus 2026":     d(2026, 8, 11),
		"11 Agu 2026":         d(2026, 8, 11),
		"3 ags 2026":          d(2026, 8, 3),
		"1 MEI 2026":          d(2026, 5, 1),
		"20 Des 2025":         d(2025, 12, 20),
		"29/02/2028":          d(2028, 2, 29),
	}
	for in, want := range good {
		got, err := ParseDate(in)
		if err != nil {
			t.Errorf("ParseDate(%q) err %v", in, err)
			continue
		}
		if in == "" {
			if got != nil {
				t.Errorf("empty should be nil, got %v", got)
			}
			continue
		}
		if got == nil || *got != *want {
			t.Errorf("ParseDate(%q)=%v want %v", in, got, want)
		}
	}
	for _, in := range []string{"31/02/2026", "29/02/2027", "13/13/2026", "abc", "0", "80001", "11 Foo 2026", "2026-02-30", "12345678"} {
		if got, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) expected error, got %v", in, got)
		}
	}
}

func TestParseMoney(t *testing.T) {
	good := map[string]string{
		"Rp 12.500.000": "12500000", "12,500,000.00": "12500000", "12.500.000,50": "12500000.5",
		"12500000": "12500000", "Rp. 1.500": "1500", "Rp1.500.000,-": "1500000", "1500.50": "1500.5",
		"12,5": "12.5", "1,500": "1500", "0": "0", "1.2500000000000001E7": "12500000.000000001", "  Rp 100  ": "100",
	}
	for in, want := range good {
		got, err := ParseMoney(in)
		if err != nil || got == nil || got.String() != want {
			t.Errorf("ParseMoney(%q)=%v,%v want %s", in, got, err, want)
		}
	}
	if got, err := ParseMoney("  "); got != nil || err != nil {
		t.Errorf("empty: %v %v", got, err)
	}
	// "1500.123" is rejected: a lone dot followed by 3 digits means thousands
	// grouping, which needs at most 3 leading digits.
	for _, in := range []string{"abc", "1.2.3,4,5", "-100", "Rp -5.000", "(1.000)", "12,50,000", "Rp", "1..5", ".", "12abc", "1500.123"} {
		if got, err := ParseMoney(in); err == nil {
			t.Errorf("ParseMoney(%q) expected error, got %v", in, got)
		}
	}
}

func TestParseWarrantyMonths(t *testing.T) {
	good := map[string]int{"12": 12, "12 bulan": 12, "12 months": 12, "1 tahun": 12, "2 years": 24, "24Bulan": 24, "12.0": 12, "0": 0}
	for in, want := range good {
		got, err := ParseWarrantyMonths(in)
		if err != nil || got == nil || *got != want {
			t.Errorf("ParseWarrantyMonths(%q)=%v,%v want %d", in, got, err, want)
		}
	}
	if got, err := ParseWarrantyMonths(""); got != nil || err != nil {
		t.Errorf("empty: %v %v", got, err)
	}
	for _, in := range []string{"abc", "-3", "12 hari", "1.5 tahun", "99999999999999999999"} {
		if got, err := ParseWarrantyMonths(in); err == nil {
			t.Errorf("ParseWarrantyMonths(%q) expected error, got %d", in, *got)
		}
	}
}
