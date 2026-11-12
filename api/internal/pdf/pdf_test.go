package pdf_test

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/civil"

	"assetku/api/internal/pdf"
)

var pageRe = regexp.MustCompile(`/Type /Page\b[^s]`)

func pages(b []byte) int { return len(pageRe.FindAll(b, -1)) }

func TestLabelPageSize90x50(t *testing.T) {
	var buf bytes.Buffer
	err := pdf.Label(&buf, pdf.LabelData{QRURL: "http://x/assets/1", Tag: "AST-8F2K1Q9Z", Name: "Laptop", Model: "X1", Serial: "SN1", TenantName: "PT Contoh"})
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`/MediaBox \[([\d. ]+)\]`).FindSubmatch(buf.Bytes())
	if m == nil {
		t.Fatal("no MediaBox")
	}
	f := strings.Fields(string(m[1]))
	w, _ := strconv.ParseFloat(f[2], 64)
	h, _ := strconv.ParseFloat(f[3], 64)
	if abs(w-255.12) > 0.1 || abs(h-141.73) > 0.1 {
		t.Fatalf("MediaBox %v, want ~[0 0 255.12 141.73]", f)
	}
	if pages(buf.Bytes()) != 1 {
		t.Fatalf("pages = %d", pages(buf.Bytes()))
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestLabelLongUnicodeName(t *testing.T) {
	long := strings.Repeat("Péré & \"Zoë\" ", 20)[:200]
	var buf bytes.Buffer
	err := pdf.Label(&buf, pdf.LabelData{QRURL: "http://x/assets/1", Tag: "AST-8F2K1Q9Z", Name: long, Model: long, Serial: long, TenantName: long})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	if pages(buf.Bytes()) != 1 {
		t.Fatalf("pages = %d", pages(buf.Bytes()))
	}
}

func TestBASTLongUnicode(t *testing.T) {
	long := strings.Repeat("Péré & \"Zoë\" ", 30)
	var buf bytes.Buffer
	err := pdf.BAST(&buf, pdf.BASTData{
		Number: "BAST-AST-8F2K1Q9Z-20261111", TenantName: long, Date: civil.Date{Year: 2026, Month: 11, Day: 11},
		FirstName: long, FirstPosition: "Manajer", SecondName: long, SecondPosition: long, SecondDepartment: long,
		Tag: "AST-8F2K1Q9Z", AssetName: long, Category: "Peralatan IT", BrandModel: long, Serial: long,
		Condition: "Baik dan Layak Digunakan", Warranty: "s.d. 11 November 2027", Notes: long,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) || pages(buf.Bytes()) > 2 {
		t.Fatalf("pages = %d", pages(buf.Bytes()))
	}
}

// TestWriteSamples writes sample documents for visual inspection when
// PDF_SAMPLE_DIR is set.
func TestWriteSamples(t *testing.T) {
	dir := os.Getenv("PDF_SAMPLE_DIR")
	if dir == "" {
		t.Skip("PDF_SAMPLE_DIR not set")
	}
	var l, b bytes.Buffer
	long := strings.Repeat("Péré & \"Zoë\" ", 20)
	if err := pdf.Label(&l, pdf.LabelData{QRURL: "http://localhost:3000/assets/0b1c", Tag: "AST-8F2K1Q9Z", Name: long, Model: "ThinkPad X1 Carbon Gen 11", Serial: "PF3ABC12345", TenantName: "PT Contoh Sejahtera"}); err != nil {
		t.Fatal(err)
	}
	if err := pdf.BAST(&b, pdf.BASTData{
		Number: "BAST-AST-8F2K1Q9Z-20261111", TenantName: "PT Contoh Sejahtera", Date: civil.Date{Year: 2026, Month: 11, Day: 11},
		FirstName: "Budi Santoso", FirstPosition: "Kepala Divisi Umum", SecondName: "Siti Rahma", SecondPosition: "Staf Keuangan", SecondDepartment: "Keuangan",
		Tag: "AST-8F2K1Q9Z", AssetName: "Laptop Lenovo ThinkPad X1 Carbon", Category: "Peralatan IT", BrandModel: "Lenovo ThinkPad X1 Carbon Gen 11", Serial: "PF3ABC12345",
		Condition: "Baik dan Layak Digunakan", Warranty: "s.d. 11 November 2027", Notes: "Termasuk tas dan charger asli.",
	}); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(dir+"/label.pdf", l.Bytes(), 0o644)
	_ = os.WriteFile(dir+"/bast.pdf", b.Bytes(), 0o644)
}
