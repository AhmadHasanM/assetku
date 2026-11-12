// Package export writes the asset list as CSV, XLSX or PDF.
//
// CSV and XLSX use importer.CanonicalHeaders and English enum labels so a file
// can be imported again. Dates are yyyy-mm-dd text, the purchase cost is a
// plain number with two decimals ("12500000.00"; a numeric cell in XLSX).
package export

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"

	"assetku/api/internal/asset"
	"assetku/api/internal/pdf"
)

// formulaSafe prefixes a single quote to text that a spreadsheet could read as
// a formula. Only CSV needs it: XLSX text cells are never evaluated. The
// importer removes the quote again (importer.UnquoteFormula).
func formulaSafe(s string) string {
	s = strings.TrimSpace(s)
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// textFields returns the 16 columns in CanonicalHeaders order. Cost and
// warranty are returned as plain strings; XLSX overrides them with numbers.
func textFields(r asset.ExportRow) []string {
	date, cost, warranty := "", "", ""
	if r.PurchaseDate != nil {
		date = r.PurchaseDate.String()
	}
	if r.PurchaseCost != nil {
		cost = r.PurchaseCost.StringFixed(2)
	}
	if r.WarrantyMonths != nil {
		warranty = strconv.Itoa(*r.WarrantyMonths)
	}
	return []string{
		r.AssetTag, r.Name, pdf.LocalizedCategory("en", r.Category), r.Manufacturer, r.Model, r.SerialNumber,
		pdf.LocalizedStatus("en", r.Status), r.LocationName, r.AssignedTo, r.OrganizationName, r.SupplierName,
		r.OrderNumber, date, cost, warranty, r.Notes,
	}
}

// numeric columns that are not formula-prefixed (cost, warranty, date are generated, never user text).
var csvPlain = map[int]bool{12: true, 13: true, 14: true}

// WriteCSV writes UTF-8 with a BOM and comma separators.
func WriteCSV(w io.Writer, headers []string, rows []asset.ExportRow) error {
	if _, err := io.WriteString(w, "\xEF\xBB\xBF"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, r := range rows {
		f := textFields(r)
		for i := range f {
			if !csvPlain[i] {
				f[i] = formulaSafe(f[i])
			}
		}
		if err := cw.Write(f); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
