// Package importer parses spreadsheet files (csv, xlsx, xls) into asset rows.
// This part covers header recognition, value parsing, and file reading.
package importer

import (
	"strings"
	"unicode"
)

// NormalizeHeader lowercases s and removes every character that is not a
// letter or a digit, so "Nama_Asset " becomes "namaasset".
func NormalizeHeader(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FieldAliases lists the accepted header names per asset field (spec section 10
// plus the Snipe-IT export headers). Keys match the asset JSON field names.
var FieldAliases = map[string][]string{
	"name":            {"Asset Name", "Nama Asset", "Nama Aset", "Nama Barang", "Name", "Item Name"},
	"asset_tag":       {"Asset Tag", "Tag", "Kode Asset", "Kode Aset", "Nomor Asset", "Asset Code"},
	"category":        {"Category", "Kategori", "Jenis"},
	"manufacturer":    {"Manufacturer", "Merek", "Merk", "Brand", "Pabrikan"},
	"model":           {"Model", "Tipe", "Type", "Model No.", "Model Number"},
	"serial_number":   {"Serial Number", "Serial", "No Seri", "Nomor Seri", "SN"},
	"status":          {"Status", "Kondisi"},
	"location":        {"Location", "Lokasi", "Default Location"},
	"assigned_to":     {"Assigned To", "Pemegang", "Pengguna", "Checked Out To", "PIC"},
	"organization":    {"Organization", "Organisasi", "Company", "Perusahaan", "Cabang", "Branch"},
	"supplier":        {"Supplier", "Vendor", "Pemasok"},
	"order_number":    {"Order Number", "No PO", "Nomor PO", "PO Number"},
	"purchase_date":   {"Purchase Date", "Tanggal Beli", "Tanggal Pembelian"},
	"purchase_cost":   {"Purchase Cost", "Harga", "Harga Beli", "Harga Pembelian"},
	"warranty_months": {"Warranty", "Warranty Months", "Garansi", "Garansi (Bulan)"},
	"notes":           {"Notes", "Catatan", "Keterangan"},
}

// aliasToField maps a normalized alias to its field key.
var aliasToField = func() map[string]string {
	m := make(map[string]string)
	for field, aliases := range FieldAliases {
		m[NormalizeHeader(field)] = field
		for _, a := range aliases {
			m[NormalizeHeader(a)] = field
		}
	}
	return m
}()

// MapHeaders maps field keys to column indexes. A header that is not
// recognized, or that repeats an already mapped field, is returned in unknown
// (the first column wins). Blank headers are ignored.
func MapHeaders(row []string) (cols map[string]int, unknown []string) {
	cols = make(map[string]int)
	for i, h := range row {
		n := NormalizeHeader(h)
		if n == "" {
			continue
		}
		field, ok := aliasToField[n]
		if !ok {
			unknown = append(unknown, strings.TrimSpace(h))
			continue
		}
		if _, dup := cols[field]; dup {
			unknown = append(unknown, strings.TrimSpace(h))
			continue
		}
		cols[field] = i
	}
	return cols, unknown
}

const headerScanRows = 10

// FindHeaderRow returns the index of the header row within the top 10 rows:
// the first row that maps a "name" column, else the first row with at least
// one recognized header.
func FindHeaderRow(rows [][]string) (idx int, ok bool) {
	first := -1
	for i := 0; i < len(rows) && i < headerScanRows; i++ {
		cols, _ := MapHeaders(rows[i])
		if len(cols) == 0 {
			continue
		}
		if _, hasName := cols["name"]; hasName {
			return i, true
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		return first, true
	}
	return 0, false
}
