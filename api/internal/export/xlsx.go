package export

import (
	"io"

	"github.com/xuri/excelize/v2"

	"assetku/api/internal/asset"
)

const sheetName = "Assets"

// WriteXLSX writes one sheet. Text goes into string cells (never evaluated as
// formulas, so no quote prefix); cost and warranty months are numeric cells.
func WriteXLSX(w io.Writer, headers []string, rows []asset.ExportRow) error {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName(f.GetSheetName(0), sheetName); err != nil {
		return err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return err
	}
	if err := sw.SetColWidth(1, len(headers), 18); err != nil {
		return err
	}
	hr := make([]any, len(headers))
	for i, h := range headers {
		hr[i] = excelize.Cell{StyleID: bold, Value: h}
	}
	if err := sw.SetRow("A1", hr); err != nil {
		return err
	}
	for n, r := range rows {
		t := textFields(r)
		row := make([]any, len(t))
		for i, v := range t {
			row[i] = v
		}
		if r.PurchaseCost != nil {
			c, _ := r.PurchaseCost.Float64()
			row[13] = c
		} else {
			row[13] = nil
		}
		if r.WarrantyMonths != nil {
			row[14] = *r.WarrantyMonths
		} else {
			row[14] = nil
		}
		cell, err := excelize.CoordinatesToCellName(1, n+2)
		if err != nil {
			return err
		}
		if err := sw.SetRow(cell, row); err != nil {
			return err
		}
	}
	if err := sw.Flush(); err != nil {
		return err
	}
	return f.Write(w)
}
