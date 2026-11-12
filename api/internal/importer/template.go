package importer

import (
	"io"

	"github.com/xuri/excelize/v2"
)

// CanonicalHeaders are the English column headers in asset field order (spec
// section 5). They are used by the import template and by the export, and
// every one of them is recognized by MapHeaders.
var CanonicalHeaders = []string{
	"Asset Tag", "Asset Name", "Category", "Manufacturer", "Model", "Serial Number", "Status",
	"Location", "Assigned To", "Organization", "Supplier", "Order Number", "Purchase Date",
	"Purchase Cost", "Warranty Months", "Notes",
}

// templateExample is the sample row under the headers, one value per header.
var templateExample = []any{
	"", "Laptop Dell Latitude 5420", "IT Equipment", "Dell", "Latitude 5420", "SN-123456", "Available",
	"Head Office", "Budi Santoso", "", "PT Contoh Supplier", "PO-2026-001", "2026-01-15",
	12500000, 24, "Leave Asset Tag empty to generate one",
}

const templateSheet = "Assets"

// WriteTemplate writes the import template: one sheet with CanonicalHeaders
// and one example row.
func WriteTemplate(w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName(f.GetSheetName(0), templateSheet); err != nil {
		return err
	}
	headers := make([]any, len(CanonicalHeaders))
	for i, h := range CanonicalHeaders {
		headers[i] = h
	}
	if err := f.SetSheetRow(templateSheet, "A1", &headers); err != nil {
		return err
	}
	if err := f.SetSheetRow(templateSheet, "A2", &templateExample); err != nil {
		return err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	last, err := excelize.CoordinatesToCellName(len(CanonicalHeaders), 1)
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(templateSheet, "A1", last, bold); err != nil {
		return err
	}
	if err := f.SetColWidth(templateSheet, "A", "P", 18); err != nil {
		return err
	}
	return f.Write(w)
}
