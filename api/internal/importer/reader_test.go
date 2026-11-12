package importer

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestReadCSVSemicolonBOM(t *testing.T) {
	in := "\xef\xbb\xbf\n Nama Asset ;Harga;Catatan\r\nLaptop ; Rp 12.500.000 ;\"a;b\"\r\n"
	rows, err := ReadFile("DATA.CSV", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"Nama Asset", "Harga", "Catatan"}, {"Laptop", "Rp 12.500.000", "a;b"}}
	// leading blank line is skipped by encoding/csv
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows=%q", rows)
	}
}

func TestReadCSVCommaRaggedLazyQuotes(t *testing.T) {
	in := "Name,Notes\nLaptop,15\" screen\nMouse\n"
	rows, err := ReadFile("a.csv", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][1] != `15" screen` || len(rows[2]) != 1 {
		t.Errorf("rows=%q", rows)
	}
}

func TestReadXLSXWithDateCell(t *testing.T) {
	f := excelize.NewFile()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.SetSheetRow("Sheet1", "A1", &[]any{"LAPORAN ASSET PT X"}))
	must(f.SetSheetRow("Sheet1", "A3", &[]any{" Nama Asset ", "Tanggal Beli", "Harga"}))
	must(f.SetSheetRow("Sheet1", "A4", &[]any{"Laptop ", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), 12500000}))
	st, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	must(err)
	must(f.SetCellStyle("Sheet1", "B4", "B4", st))
	_, err = f.NewSheet("Second")
	must(err)
	must(f.SetCellValue("Second", "A1", "ignored"))
	var buf bytes.Buffer
	must(f.Write(&buf))

	rows, err := ReadFile("x.XLSX", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows=%q", rows)
	}
	idx, ok := FindHeaderRow(rows)
	if !ok || idx != 2 {
		t.Fatalf("header idx=%d ok=%v", idx, ok)
	}
	cols, _ := MapHeaders(rows[idx])
	data := rows[3]
	if data[cols["name"]] != "Laptop" {
		t.Errorf("name=%q", data[cols["name"]])
	}
	date, err := ParseDate(data[cols["purchase_date"]])
	if err != nil || date == nil || date.String() != "2026-01-15" {
		t.Errorf("date cell %q -> %v, %v", data[cols["purchase_date"]], date, err)
	}
	cost, err := ParseMoney(data[cols["purchase_cost"]])
	if err != nil || cost == nil || cost.String() != "12500000" {
		t.Errorf("cost cell %q -> %v, %v", data[cols["purchase_cost"]], cost, err)
	}
}

func TestReadXLS(t *testing.T) {
	fh, err := os.Open("testdata/sample.xls")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	rows, err := ReadFile("sample.xls", fh)
	if err != nil {
		t.Fatal(err)
	}
	idx, ok := FindHeaderRow(rows)
	if !ok || idx != 2 {
		t.Fatalf("header idx=%d ok=%v rows=%q", idx, ok, rows)
	}
	cols, _ := MapHeaders(rows[idx])
	data := rows[idx+1]
	if data[cols["name"]] != "Laptop" {
		t.Errorf("name=%q", data[cols["name"]])
	}
	// serial stored as a number with a custom format must stay digits
	if got := data[cols["serial_number"]]; got != "1234567890" {
		t.Errorf("serial=%q", got)
	}
	date, err := ParseDate(data[cols["purchase_date"]])
	if err != nil || date == nil || date.String() != "2026-01-15" {
		t.Errorf("date %q -> %v, %v", data[cols["purchase_date"]], date, err)
	}
	// price is a number formatted as "Rp" #,##0
	if got := data[cols["purchase_cost"]]; got != "12500000" {
		t.Errorf("cost cell=%q", got)
	}
	cost, err := ParseMoney(data[cols["purchase_cost"]])
	if err != nil || cost == nil || cost.String() != "12500000" {
		t.Errorf("cost -> %v, %v", cost, err)
	}
}

func TestReadFileErrors(t *testing.T) {
	if _, err := ReadFile("a.txt", strings.NewReader("x")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("txt: %v", err)
	}
	if _, err := ReadFile("noext", strings.NewReader("x")); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("noext: %v", err)
	}
	for _, name := range []string{"a.xlsx", "a.xls"} {
		if _, err := ReadFile(name, strings.NewReader("this is not a spreadsheet")); err == nil {
			t.Errorf("%s garbage: expected error", name)
		}
	}
}
