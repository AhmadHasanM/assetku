package pdf

import (
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/civil"
)

var (
	units    = []string{"nol", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas"}
	hariID   = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	bulanID  = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	bigScale = []struct {
		v    int
		name string
	}{{1_000_000_000_000, "triliun"}, {1_000_000_000, "miliar"}, {1_000_000, "juta"}, {1_000, "ribu"}}
)

// Terbilang spells a non-negative integer in Indonesian ("dua ribu dua puluh enam").
// Negative numbers are prefixed with "minus".
func Terbilang(n int) string {
	if n < 0 {
		return "minus " + Terbilang(-n)
	}
	if n == 0 {
		return units[0]
	}
	return strings.TrimSpace(spell(n))
}

func spell(n int) string {
	switch {
	case n < 12:
		return units[n]
	case n < 20:
		return units[n-10] + " belas"
	case n < 100:
		return strings.TrimSpace(units[n/10] + " puluh " + zeroEmpty(n%10))
	case n < 200:
		return strings.TrimSpace("seratus " + zeroEmpty(n-100))
	case n < 1000:
		return strings.TrimSpace(units[n/100] + " ratus " + zeroEmpty(n%100))
	}
	for _, s := range bigScale {
		if n < s.v {
			continue
		}
		head := n / s.v
		rest := n % s.v
		var h string
		if s.v == 1000 && head == 1 {
			h = "seribu"
		} else {
			h = spell(head) + " " + s.name
		}
		return strings.TrimSpace(h + " " + zeroEmpty(rest))
	}
	return ""
}

func zeroEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return spell(n)
}

// TanggalKata renders d as "Rabu, tanggal sebelas bulan November tahun dua ribu dua puluh enam".
func TanggalKata(d civil.Date) string {
	wd := d.In(time.UTC).Weekday()
	return fmt.Sprintf("%s, tanggal %s bulan %s tahun %s",
		hariID[wd], Terbilang(d.Day), bulanID[d.Month-1], Terbilang(d.Year))
}

// FormatDateID renders d as "11 November 2027" with the Indonesian month name.
func FormatDateID(d civil.Date) string {
	return fmt.Sprintf("%d %s %d", d.Day, bulanID[d.Month-1], d.Year)
}
