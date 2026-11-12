package importer

import (
	"reflect"
	"testing"
)

func TestNormalizeHeader(t *testing.T) {
	for in, want := range map[string]string{
		"Nama_Asset ": "namaasset", "Model No.": "modelno", "Garansi (Bulan)": "garansibulan", "": "",
	} {
		if got := NormalizeHeader(in); got != want {
			t.Errorf("NormalizeHeader(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMapHeaders(t *testing.T) {
	cols, unknown := MapHeaders([]string{"Nama_Asset", "Harga Pembelian", "Kolom Aneh", "", "Checked Out To", "Asset Name", "Order Number", "Serial", "Default Location"})
	want := map[string]int{"name": 0, "purchase_cost": 1, "assigned_to": 4, "order_number": 6, "serial_number": 7, "location": 8}
	if !reflect.DeepEqual(cols, want) {
		t.Errorf("cols=%v want %v", cols, want)
	}
	if !reflect.DeepEqual(unknown, []string{"Kolom Aneh", "Asset Name"}) {
		t.Errorf("unknown=%v", unknown)
	}
}

func TestEveryAliasMapsToItsField(t *testing.T) {
	for field, aliases := range FieldAliases {
		for _, a := range aliases {
			cols, _ := MapHeaders([]string{a})
			if i, ok := cols[field]; !ok || i != 0 {
				t.Errorf("alias %q did not map to %s: %v", a, field, cols)
			}
		}
	}
}

func TestFindHeaderRow(t *testing.T) {
	rows := [][]string{{"LAPORAN ASSET PT X"}, {}, {"Nama Asset", "Kategori"}, {"Laptop", "IT"}}
	if i, ok := FindHeaderRow(rows); !ok || i != 2 {
		t.Errorf("got %d,%v", i, ok)
	}
	// name row wins over an earlier row with only another header
	rows = [][]string{{"Kategori"}, {"Nama Asset", "Kategori"}}
	if i, ok := FindHeaderRow(rows); !ok || i != 1 {
		t.Errorf("name priority: got %d,%v", i, ok)
	}
	rows = [][]string{{"Kategori"}, {"foo"}}
	if i, ok := FindHeaderRow(rows); !ok || i != 0 {
		t.Errorf("fallback: got %d,%v", i, ok)
	}
	if _, ok := FindHeaderRow([][]string{{"x"}, {"y"}}); ok {
		t.Error("expected no header")
	}
	// header beyond the top 10 rows is not found
	far := make([][]string, 11)
	far[10] = []string{"Name"}
	if _, ok := FindHeaderRow(far); ok {
		t.Error("row 11 must be ignored")
	}
}
