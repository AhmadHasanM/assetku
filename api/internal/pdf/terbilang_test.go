package pdf

import (
	"testing"

	"cloud.google.com/go/civil"
)

func TestTerbilang(t *testing.T) {
	for n, want := range map[int]string{
		0: "nol", 5: "lima", 11: "sebelas", 12: "dua belas", 15: "lima belas", 21: "dua puluh satu",
		100: "seratus", 101: "seratus satu", 111: "seratus sebelas", 250: "dua ratus lima puluh",
		1000: "seribu", 1001: "seribu satu", 2000: "dua ribu", 2026: "dua ribu dua puluh enam",
		11000: "sebelas ribu", 100000: "seratus ribu", 1000000: "satu juta",
	} {
		if got := Terbilang(n); got != want {
			t.Errorf("Terbilang(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestTanggalKata(t *testing.T) {
	got := TanggalKata(civil.Date{Year: 2026, Month: 11, Day: 11})
	want := "Rabu, tanggal sebelas bulan November tahun dua ribu dua puluh enam"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatDateID(t *testing.T) {
	if got := FormatDateID(civil.Date{Year: 2027, Month: 11, Day: 11}); got != "11 November 2027" {
		t.Fatalf("got %q", got)
	}
}
