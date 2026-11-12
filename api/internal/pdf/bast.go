package pdf

import (
	"io"

	"cloud.google.com/go/civil"
	"github.com/go-pdf/fpdf"
)

// BASTData is the content of a handover document (Berita Acara Serah Terima).
// Category and Condition are Indonesian display labels.
type BASTData struct {
	Number, TenantName                           string
	Date                                         civil.Date
	FirstName, FirstPosition                     string
	SecondName, SecondPosition, SecondDepartment string
	Tag, AssetName, Category, BrandModel, Serial string
	Condition, Warranty, Notes                   string
}

// CategoryLabel maps a category code to its Indonesian label.
func CategoryLabel(code string) string {
	switch code {
	case "it_equipment":
		return "Peralatan IT"
	case "vehicle":
		return "Kendaraan"
	case "building_facility":
		return "Fasilitas Gedung"
	case "furniture":
		return "Furnitur"
	case "electronics":
		return "Elektronik"
	case "other":
		return "Lainnya"
	}
	return code
}

// ConditionLabel maps a BAST condition code to its Indonesian label.
// ok is false for an unknown code.
func ConditionLabel(code string) (label string, ok bool) {
	switch code {
	case "good":
		return "Baik dan Layak Digunakan", true
	case "new_sealed":
		return "Baru (Segel)", true
	case "used_normal":
		return "Bekas (Normal)", true
	}
	return "", false
}

const (
	pageMargin  = 20.0
	contentW    = 210 - 2*pageMargin
	lh          = 5.6 // body line height
	maxNotes    = 800 // runes of notes kept in the document
	maxWarranty = 200 // runes of warranty text kept in the document
)

func dash(s string) string {
	if c := clean(s); c != "" {
		return c
	}
	return "-"
}

// BAST writes an A4 portrait handover document in Indonesian.
func BAST(w io.Writer, d BASTData) error {
	p := newDoc("P", "A4")
	p.SetMargins(pageMargin, pageMargin, pageMargin)
	p.AddPage()
	p.SetTextColor(0, 0, 0)
	p.SetDrawColor(0, 0, 0)

	// Letterhead.
	p.SetFont(fontBody, "B", 15)
	p.SetXY(pageMargin, pageMargin)
	p.CellFormat(contentW, 7, fit(p, d.TenantName, contentW), "", 1, "C", false, 0, "")
	p.SetLineWidth(0.6)
	p.Line(pageMargin, p.GetY()+1.5, pageMargin+contentW, p.GetY()+1.5)
	p.SetLineWidth(0.2)
	p.Line(pageMargin, p.GetY()+2.3, pageMargin+contentW, p.GetY()+2.3)

	// Title and number.
	p.SetY(p.GetY() + 8)
	p.SetFont(fontBody, "B", 13)
	p.CellFormat(contentW, 6.5, "BERITA ACARA SERAH TERIMA BARANG", "", 1, "C", false, 0, "")
	p.SetFont(fontMono, "", 10)
	p.CellFormat(contentW, 6, fit(p, "Nomor: "+d.Number, contentW), "", 1, "C", false, 0, "")

	// Opening paragraph.
	p.SetY(p.GetY() + 5)
	p.SetFont(fontBody, "", 10.5)
	p.MultiCell(contentW, lh, "Pada hari ini, "+TanggalKata(d.Date)+", yang bertanda tangan di bawah ini:", "", "L", false)
	p.SetY(p.GetY() + 2)

	party(p, "1.", "PIHAK PERTAMA", d.FirstName, d.FirstPosition, "")
	party(p, "2.", "PIHAK KEDUA", d.SecondName, d.SecondPosition, d.SecondDepartment)

	p.SetFont(fontBody, "", 10.5)
	p.MultiCell(contentW, lh, "PIHAK PERTAMA menyerahkan kepada PIHAK KEDUA barang dengan rincian sebagai berikut:", "", "L", false)
	p.SetY(p.GetY() + 2.5)

	itemTable(p, d)

	// Warranty and notes.
	p.SetY(p.GetY() + 4)
	labelled(p, "Garansi", dash(capRunes(clean(d.Warranty), maxWarranty)))
	labelled(p, "Catatan", dash(capRunes(clean(d.Notes), maxNotes)))

	// Handover statement.
	p.SetY(p.GetY() + 4)
	p.SetFont(fontBody, "", 10.5)
	p.MultiCell(contentW, lh,
		"Dengan ditandatanganinya berita acara ini, barang tersebut telah diserahkan oleh PIHAK PERTAMA dan "+
			"diterima oleh PIHAK KEDUA dalam kondisi sebagaimana tercantum di atas. Sejak serah terima ini, barang "+
			"berada dalam penguasaan dan tanggung jawab PIHAK KEDUA. Demikian berita acara ini dibuat dengan sebenarnya "+
			"untuk dipergunakan sebagaimana mestinya.", "", "J", false)

	signatures(p, d)
	return p.Output(w)
}

// party writes one numbered identity block.
func party(p *fpdf.Fpdf, num, role, name, position, dept string) {
	const labelW = 30.0
	p.SetFont(fontBody, "B", 10.5)
	p.SetX(pageMargin)
	p.CellFormat(8, lh, num, "", 0, "L", false, 0, "")
	p.CellFormat(contentW-8, lh, role, "", 1, "L", false, 0, "")
	rows := [][2]string{{"Nama", name}, {"Jabatan", position}}
	if dept != "" || role == "PIHAK KEDUA" {
		rows = append(rows, [2]string{"Departemen", dept})
	}
	for _, r := range rows {
		p.SetFont(fontBody, "", 10.5)
		p.SetX(pageMargin + 8)
		p.CellFormat(labelW, lh, r[0], "", 0, "L", false, 0, "")
		p.CellFormat(4, lh, ":", "", 0, "L", false, 0, "")
		valW := contentW - 8 - labelW - 4
		lines := wrapCap(p, dash(r[1]), valW, 2)
		for i, l := range lines {
			if i > 0 {
				p.SetX(pageMargin + 8 + labelW + 4)
			}
			p.CellFormat(valW, lh, l, "", 1, "L", false, 0, "")
		}
	}
	p.SetY(p.GetY() + 2.5)
}

// labelled writes "Label : value" with a wrapped value.
func labelled(p *fpdf.Fpdf, label, value string) {
	const labelW = 24.0
	p.SetFont(fontBody, "B", 10.5)
	y := p.GetY()
	p.SetXY(pageMargin, y)
	p.CellFormat(labelW, lh, label, "", 0, "L", false, 0, "")
	p.CellFormat(4, lh, ":", "", 0, "L", false, 0, "")
	p.SetFont(fontBody, "", 10.5)
	p.SetXY(pageMargin+labelW+4, y)
	p.MultiCell(contentW-labelW-4, lh, value, "", "L", false)
}

type col struct {
	head string
	w    float64
	mono bool
}

// itemTable draws the single-row item table with wrapped, capped cells.
func itemTable(p *fpdf.Fpdf, d BASTData) {
	cols := []col{
		{"Asset Tag", 31, true}, {"Nama Barang", 36, false}, {"Kategori", 24, false},
		{"Merek / Model", 30, false}, {"Nomor Seri", 27, true}, {"Kondisi", 22, false},
	}
	vals := []string{d.Tag, d.AssetName, d.Category, d.BrandModel, d.Serial, d.Condition}
	const pad, maxLines, headH, cellH = 1.5, 4, 8.0, 4.6

	x0, y0 := pageMargin, p.GetY()
	p.SetFillColor(235, 235, 235)
	p.SetFont(fontBody, "B", 9)
	x := x0
	for _, c := range cols {
		p.Rect(x, y0, c.w, headH, "DF")
		p.SetXY(x+pad, y0)
		p.CellFormat(c.w-2*pad, headH, c.head, "", 0, "L", false, 0, "")
		x += c.w
	}

	lines := make([][]string, len(cols))
	most := 1
	for i, c := range cols {
		if c.mono {
			p.SetFont(fontMono, "", 7.5)
		} else {
			p.SetFont(fontBody, "", 9)
		}
		lines[i] = wrapCap(p, vals[i], c.w-2*pad, maxLines)
		most = max(most, len(lines[i]))
	}
	rowH := float64(most)*cellH + 2*pad
	y := y0 + headH
	x = x0
	for i, c := range cols {
		p.Rect(x, y, c.w, rowH, "D")
		if c.mono {
			p.SetFont(fontMono, "", 7.5)
		} else {
			p.SetFont(fontBody, "", 9)
		}
		for j, l := range lines[i] {
			p.SetXY(x+pad, y+pad+float64(j)*cellH)
			p.CellFormat(c.w-2*pad, cellH, l, "", 0, "L", false, 0, "")
		}
		x += c.w
	}
	p.SetY(y + rowH)
}

// signatures draws the two signature columns side by side, moving to a new
// page when they would not fit.
func signatures(p *fpdf.Fpdf, d BASTData) {
	const blockH = 56.0
	if p.GetY()+8+blockH > 297-pageMargin {
		p.AddPage()
		p.SetY(pageMargin)
	} else {
		p.SetY(p.GetY() + 8)
	}
	y := p.GetY()
	colW := contentW / 2
	cols := []struct{ role, name, position string }{
		{"PIHAK PERTAMA", d.FirstName, d.FirstPosition},
		{"PIHAK KEDUA", d.SecondName, d.SecondPosition},
	}
	for i, c := range cols {
		x := pageMargin + float64(i)*colW
		inner := colW - 10
		p.SetFont(fontBody, "B", 10.5)
		p.SetXY(x+5, y)
		p.CellFormat(inner, lh, c.role, "", 0, "C", false, 0, "")
		// Space for the signature.
		p.SetXY(x+5, y+lh+24)
		p.SetFont(fontBody, "B", 10.5)
		p.CellFormat(inner, lh, fit(p, dash(c.name), inner), "", 0, "C", false, 0, "")
		ly := y + 2*lh + 24
		p.SetLineWidth(0.3)
		p.Line(x+8, ly, x+colW-8, ly)
		p.SetFont(fontBody, "", 9.5)
		p.SetXY(x+5, ly+1)
		p.CellFormat(inner, lh, fit(p, dash(c.position), inner), "", 0, "C", false, 0, "")
	}
}

// capRunes keeps at most n runes of s, marking a cut with "…".
func capRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
