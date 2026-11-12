// Package pdf renders asset labels and BAST documents.
package pdf

import (
	"embed"
	"strings"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

// Font families.
const (
	fontBody = "dmsans"
	fontMono = "spacemono"
)

// newDoc creates a PDF with the embedded fonts registered. Cell margins are
// zero so that text width equals the width passed to GetStringWidth.
func newDoc(orientation string, size any) *fpdf.Fpdf {
	var p *fpdf.Fpdf
	switch s := size.(type) {
	case string:
		p = fpdf.New(orientation, "mm", s, "")
	case fpdf.SizeType:
		p = fpdf.NewCustom(&fpdf.InitType{OrientationStr: orientation, UnitStr: "mm", Size: s})
	}
	for _, f := range []struct{ family, style, file string }{
		{fontBody, "", "DMSans-Regular.ttf"},
		{fontBody, "B", "DMSans-Bold.ttf"},
		{fontMono, "", "SpaceMono-Regular.ttf"},
		{fontMono, "B", "SpaceMono-Bold.ttf"},
	} {
		b, err := fontFS.ReadFile("fonts/" + f.file)
		if err != nil {
			p.SetError(err)
			return p
		}
		p.AddUTF8FontFromBytes(f.family, f.style, b)
	}
	p.SetCellMargin(0)
	p.SetAutoPageBreak(false, 0)
	p.SetCreator("AssetKu", false)
	return p
}

// clean removes control characters that the PDF text path cannot render.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

// fit trims s so that it is at most w mm wide in the current font, adding an
// ellipsis when anything was cut.
func fit(p *fpdf.Fpdf, s string, w float64) string {
	s = clean(s)
	if p.GetStringWidth(s) <= w {
		return s
	}
	return cut(p, s, w)
}

// cut always appends an ellipsis, trimming s until the result fits in w.
func cut(p *fpdf.Fpdf, s string, w float64) string {
	r := []rune(strings.TrimRight(s, " "))
	for len(r) > 0 && p.GetStringWidth(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return strings.TrimRight(string(r), " ") + "…"
}

// wrapCap wraps s to width w and keeps at most maxLines lines; when text is
// dropped the last line ends with an ellipsis.
func wrapCap(p *fpdf.Fpdf, s string, w float64, maxLines int) []string {
	s = clean(s)
	if s == "" {
		return []string{"-"}
	}
	raw := p.SplitText(s, w)
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, strings.TrimRight(l, " "))
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = cut(p, lines[maxLines-1], w)
	}
	return lines
}
