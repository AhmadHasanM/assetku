package pdf

import (
	"fmt"
	"io"
)

// AssetListRow is one table row of the asset list PDF. It is a local type
// because package asset imports pdf (asset.ListItem cannot be used here).
type AssetListRow struct {
	AssetTag, Name, Category, Model, SerialNumber, Status, LocationName, AssignedTo string
}

// AssetListData is the content of the asset list export. Locale is "id" or "en".
type AssetListData struct {
	TenantName, PrintedAt, FilterSummary string
	Rows                                 []AssetListRow
	Locale                               string
}

var categoryLabels = map[string][2]string{
	"it_equipment":      {"Peralatan IT", "IT Equipment"},
	"vehicle":           {"Kendaraan", "Vehicle"},
	"building_facility": {"Fasilitas Gedung", "Building Facility"},
	"furniture":         {"Furnitur", "Furniture"},
	"electronics":       {"Elektronik", "Electronics"},
	"other":             {"Lainnya", "Other"},
}

var statusLabels = map[string][2]string{
	"available":   {"Tersedia", "Available"},
	"in_use":      {"Digunakan", "In Use"},
	"maintenance": {"Perbaikan", "Maintenance"},
	"damaged":     {"Rusak", "Damaged"},
	"lost":        {"Hilang", "Lost"},
	"archived":    {"Diarsipkan", "Archived"},
}

func label(m map[string][2]string, locale, code string) string {
	l, ok := m[code]
	if !ok {
		return code
	}
	if locale == "en" {
		return l[1]
	}
	return l[0]
}

// LocalizedCategory returns the category name for locale ("id" or "en"). The
// English labels are recognized by the importer, so exports re-import.
func LocalizedCategory(locale, code string) string { return label(categoryLabels, locale, code) }

// LocalizedStatus returns the status name for locale ("id" or "en").
func LocalizedStatus(locale, code string) string { return label(statusLabels, locale, code) }

const (
	listMargin  = 10.0
	listRowH    = 6.5
	listFontPt  = 8.0
	listPadding = 1.2
)

type listCol struct {
	id, en string
	w      float64
	mono   bool
}

// Widths add up to 277 mm, the printable width of A4 landscape.
var listCols = []listCol{
	{"Tag", "Tag", 30, true}, {"Nama", "Name", 55, false}, {"Kategori", "Category", 30, false},
	{"Model", "Model", 38, false}, {"Serial", "Serial", 36, true}, {"Status", "Status", 24, false},
	{"Lokasi", "Location", 32, false}, {"Pemegang", "Assigned To", 32, false},
}

// AssetList writes the asset list as an A4 landscape table on a white page.
// The table header repeats on every page and the footer shows page numbers.
func AssetList(w io.Writer, d AssetListData) error {
	en := d.Locale == "en"
	tr := func(id, e string) string {
		if en {
			return e
		}
		return id
	}
	p := newDoc("L", "A4")
	p.SetMargins(listMargin, listMargin, listMargin)
	p.AliasNbPages("")
	pageW, pageH := p.GetPageSize()
	contentW := pageW - 2*listMargin

	p.SetFooterFunc(func() {
		p.SetY(pageH - 8)
		p.SetFont(fontBody, "", 8)
		p.SetTextColor(90, 90, 90)
		p.CellFormat(contentW, 4, fmt.Sprintf(tr("Halaman %d dari {nb}", "Page %d of {nb}"), p.PageNo()), "", 0, "R", false, 0, "")
	})

	tableHeader := func() {
		p.SetFont(fontBody, "B", listFontPt)
		p.SetFillColor(235, 235, 240)
		p.SetTextColor(0, 0, 0)
		p.SetDrawColor(180, 180, 190)
		x := listMargin
		for _, c := range listCols {
			p.SetXY(x, p.GetY())
			p.CellFormat(c.w, listRowH, fit(p, tr(c.id, c.en), c.w-2*listPadding), "B", 0, "L", true, 0, "")
			x += c.w
		}
		p.SetY(p.GetY() + listRowH)
	}
	newPage := func() {
		p.AddPage()
		p.SetY(listMargin)
	}

	newPage()
	p.SetTextColor(0, 0, 0)
	p.SetFont(fontBody, "B", 14)
	p.SetXY(listMargin, p.GetY())
	p.CellFormat(contentW, 7, fit(p, d.TenantName, contentW), "", 0, "L", false, 0, "")
	p.SetY(p.GetY() + 7)
	p.SetFont(fontBody, "", 9)
	for _, line := range []string{
		tr("Daftar Aset", "Asset List") + "  |  " + tr("Dicetak: ", "Printed: ") + d.PrintedAt +
			"  |  " + tr("Total: ", "Total: ") + fmt.Sprint(len(d.Rows)),
		d.FilterSummary,
	} {
		p.SetXY(listMargin, p.GetY())
		p.CellFormat(contentW, 5, fit(p, line, contentW), "", 0, "L", false, 0, "")
		p.SetY(p.GetY() + 5)
	}
	p.SetY(p.GetY() + 2)
	tableHeader()

	limit := pageH - 14 // keep clear of the footer
	for i, r := range d.Rows {
		if p.GetY()+listRowH > limit {
			newPage()
			tableHeader()
		}
		if i%2 == 1 {
			p.SetFillColor(248, 248, 250)
		}
		x := listMargin
		y := p.GetY()
		vals := []string{r.AssetTag, r.Name, LocalizedCategory(d.Locale, r.Category), r.Model, r.SerialNumber,
			LocalizedStatus(d.Locale, r.Status), r.LocationName, r.AssignedTo}
		for j, c := range listCols {
			fam, size := fontBody, listFontPt
			if c.mono {
				fam, size = fontMono, 7.0
			}
			p.SetFont(fam, "", size)
			p.SetTextColor(0, 0, 0)
			p.SetXY(x, y)
			p.CellFormat(c.w, listRowH, fit(p, vals[j], c.w-2*listPadding), "", 0, "L", i%2 == 1, 0, "")
			x += c.w
		}
		p.SetY(y + listRowH)
	}
	if len(d.Rows) == 0 {
		p.SetFont(fontBody, "", 9)
		p.SetXY(listMargin, p.GetY()+2)
		p.CellFormat(contentW, 6, tr("Tidak ada aset.", "No assets."), "", 0, "L", false, 0, "")
	}
	return p.Output(w)
}
