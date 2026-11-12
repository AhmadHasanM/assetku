package pdf

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

// LabelData is the content of one asset label.
type LabelData struct {
	QRURL, Tag, Name, Model, Serial, TenantName string
}

const (
	labelW, labelH = 90.0, 50.0
	labelMargin    = 3.0
	qrSize         = 38.0
)

// Label writes a single-page 90x50 mm label with a QR code on the left and
// the asset details on the right.
func Label(w io.Writer, d LabelData) error {
	png, err := qrcode.Encode(d.QRURL, qrcode.Medium, 512)
	if err != nil {
		return fmt.Errorf("pdf: encode qr: %w", err)
	}
	p := newDoc("P", fpdf.SizeType{Wd: labelW, Ht: labelH})
	p.SetMargins(0, 0, 0)
	p.AddPage()
	p.RegisterImageOptionsReader("qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
	p.ImageOptions("qr", labelMargin, (labelH-qrSize)/2, qrSize, qrSize, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	x := labelMargin + qrSize + 3
	colW := labelW - labelMargin - x
	p.SetTextColor(0, 0, 0)
	y := 7.0
	line := func(family, style string, size, h float64, text string) {
		if clean(text) == "" {
			return
		}
		p.SetFont(family, style, size)
		p.SetXY(x, y)
		p.CellFormat(colW, h, fit(p, text, colW), "", 0, "L", false, 0, "")
		y += h
	}
	line(fontMono, "B", 11, 6, d.Tag)
	y += 1
	line(fontBody, "B", 9, 4.6, d.Name)
	line(fontBody, "", 8, 4.2, d.Model)
	if clean(d.Serial) != "" {
		line(fontMono, "", 7, 4, "S/N "+d.Serial)
	}
	// Tenant name sits at the bottom of the column.
	y = labelH - labelMargin - 3.5
	p.SetTextColor(90, 90, 90)
	line(fontBody, "", 6.5, 3.5, d.TenantName)
	return p.Output(w)
}
