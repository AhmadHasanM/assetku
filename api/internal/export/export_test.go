package export_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/export"
	"assetku/api/internal/httpx"
	"assetku/api/internal/importer"
	"assetku/api/internal/masterdata"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type env struct {
	pool *pgxpool.Pool
	a, b *testutil.APIClient
	imp  *importer.Service
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	md := masterdata.NewService(pool, as)
	assets := asset.NewService(pool, as)
	imp := importer.NewService(pool, as)
	ex := export.NewService(pool, as, assets)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		md.Register(m)
		assets.Register(m)
		imp.Register(m)
		ex.Register(m)
	}}))
	t.Cleanup(srv.Close)
	t.Cleanup(imp.Wait)
	e := &env{pool: pool, imp: imp}
	ta := testutil.CreateTenant(t, pool, "Acme")
	tb := testutil.CreateTenant(t, pool, "Other")
	testutil.CreateUser(t, pool, ta, "a@a.test", "password-1")
	testutil.CreateUser(t, pool, tb, "b@b.test", "password-1")
	e.a, e.b = testutil.Client(t, srv.URL), testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{e.a: "a@a.test", e.b: "b@b.test"} {
		res := c.Login(em, "password-1")
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("login %d", res.StatusCode)
		}
	}
	return e
}

func create(t *testing.T, c *testutil.APIClient, body map[string]any) {
	t.Helper()
	res := c.Do("POST", "/api/assets", body)
	res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("create: %d", res.StatusCode)
	}
}

func get(t *testing.T, c *testutil.APIClient, query string) (*http.Response, []byte) {
	t.Helper()
	res := c.Do("GET", "/api/assets/export?"+query, nil)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, b
}

func readCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("\xEF\xBB\xBF")) {
		t.Fatal("csv has no BOM")
	}
	recs, err := csv.NewReader(bytes.NewReader(b[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestExportRespectsFilterAllPages(t *testing.T) {
	e := setup(t)
	for i := 0; i < 45; i++ {
		st := "available"
		if i < 30 {
			st = "in_use"
		}
		create(t, e.a, map[string]any{"name": fmt.Sprintf("Asset %02d", i), "category": "it_equipment", "status": st})
	}
	res, b := get(t, e.a, "format=csv&status=in_use")
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	recs := readCSV(t, b)
	if len(recs) != 31 {
		t.Fatalf("rows = %d, want 30 + header", len(recs))
	}
	if strings.Join(recs[0], ",") != strings.Join(importer.CanonicalHeaders, ",") {
		t.Fatalf("headers %v", recs[0])
	}
	// the list endpoint is still paginated
	lres := e.a.Do("GET", "/api/assets?status=in_use", nil)
	var l struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	e.a.JSON(lres, &l)
	if len(l.Items) != 20 || l.Total != 30 {
		t.Fatalf("list items=%d total=%d", len(l.Items), l.Total)
	}
	cd := res.Header.Get("Content-Disposition")
	want := "assets-" + time.Now().In(asset.Jakarta).Format("20060102") + ".csv"
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, want) {
		t.Fatalf("Content-Disposition %q", cd)
	}
}

func TestInvalidFormatAndFilter(t *testing.T) {
	e := setup(t)
	for q, code := range map[string]string{"format=docx": "invalid_format", "": "invalid_format", "format=csv&status=bogus": "invalid_filter"} {
		res := e.a.Do("GET", "/api/assets/export?"+q, nil)
		var er struct{ Error struct{ Code string } }
		e.a.JSON(res, &er)
		if res.StatusCode != 400 || er.Error.Code != code {
			t.Errorf("%q: %d %q", q, res.StatusCode, er.Error.Code)
		}
	}
}

func doImport(t *testing.T, e *env, c *testutil.APIClient, name string, data []byte) importer.Result {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	res := c.DoRaw("POST", "/api/imports", &buf, mw.FormDataContentType())
	var out struct{ ID string }
	c.JSON(res, &out)
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("upload %d", res.StatusCode)
	}
	for end := time.Now().Add(60 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		var v struct {
			Status string
			Result *importer.Result
		}
		c.JSON(c.Do("GET", "/api/imports/"+out.ID, nil), &v)
		if v.Status == "failed" {
			t.Fatal("import failed")
		}
		if v.Status == "done" {
			return *v.Result
		}
	}
	t.Fatal("import stuck")
	return importer.Result{}
}

type fullAsset struct {
	Name           string
	Category       string
	Manufacturer   string
	Model          string
	Status         string
	Notes          string
	AssetTag       string  `json:"asset_tag"`
	SerialNumber   string  `json:"serial_number"`
	AssignedTo     string  `json:"assigned_to"`
	OrderNumber    string  `json:"order_number"`
	PurchaseDate   *string `json:"purchase_date"`
	PurchaseCost   *string `json:"purchase_cost"`
	WarrantyMonths *int    `json:"warranty_months"`
	Location       *struct{ Name string }
	Organization   *struct{ Name string }
	Supplier       *struct{ Name string }
}

func (f fullAsset) key() string {
	nm := func(r *struct{ Name string }) string {
		if r == nil {
			return ""
		}
		return r.Name
	}
	s := func(p *string) string {
		if p == nil {
			return "-"
		}
		return *p
	}
	w := -1
	if f.WarrantyMonths != nil {
		w = *f.WarrantyMonths
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s|%s", f.Name, strings.ToLower(f.AssetTag), f.Category, f.Manufacturer,
		f.Model, f.SerialNumber, f.Status, f.AssignedTo, f.OrderNumber, f.Notes, s(f.PurchaseDate), s(f.PurchaseCost), nm(f.Location),
		nm(f.Supplier), w, nm(f.Organization), "")
}

func fetchAll(t *testing.T, c *testutil.APIClient) map[string]string {
	t.Helper()
	out := map[string]string{}
	for page := 1; page <= 5; page++ {
		var l struct {
			Items []struct {
				ID       string
				AssetTag string `json:"asset_tag"`
			}
		}
		c.JSON(c.Do("GET", fmt.Sprintf("/api/assets?sort=asset_tag&dir=asc&page=%d", page), nil), &l)
		for _, it := range l.Items {
			var f fullAsset
			c.JSON(c.Do("GET", "/api/assets/"+it.ID, nil), &f)
			out[strings.ToLower(it.AssetTag)] = f.key()
		}
	}
	return out
}

func seedRich(t *testing.T, e *env) {
	t.Helper()
	// the importer auto-creates locations and suppliers
	rows := [][]string{
		{"AST-RT000001", "Laptop \"Dell\", 5420", "Peralatan IT", "Dell", "5420", "SN1", "Digunakan", "Gudang A", "Budi", "", "Vendor X", "PO-1", "2026-01-15", "12500000.50", "24", "line1\nline2"},
		{"AST-RT000002", "Mobil é & ü", "Kendaraan", "Toyota", "Avanza", "", "Tersedia", "", "", "", "", "", "", "250000000", "", "=1+1 formula"},
		{"AST-RT000003", "Meja", "Furniture", "", "", "", "Rusak", "Kantor", "", "", "", "", "2025-12-31", "1500", "12", "-minus"},
		{"AST-RT000004", "+plus", "Other", "", "", "", "Lost", "", "@who", "", "", "", "", "0.5", "0", "@mention"},
	}
	var buf bytes.Buffer
	buf.WriteString(strings.Join(importer.CanonicalHeaders, ",") + "\n")
	w := csv.NewWriter(&buf)
	_ = w.WriteAll(rows)
	res := doImport(t, e, e.a, "seed.csv", buf.Bytes())
	if res.Created != 4 {
		t.Fatalf("seed: %+v", res)
	}
}

func TestCSVRoundTripImport(t *testing.T) {
	e := setup(t)
	seedRich(t, e)
	for i := 0; i < 20; i++ {
		create(t, e.a, map[string]any{"name": fmt.Sprintf("Plain %02d", i), "category": "electronics", "status": "maintenance"})
	}
	want := fetchAll(t, e.a)
	if len(want) != 24 {
		t.Fatalf("seeded %d: %v", len(want), want)
	}
	res, b := get(t, e.a, "format=csv")
	if res.StatusCode != 200 {
		t.Fatalf("export %d", res.StatusCode)
	}
	// formula-looking cells are quoted in the file
	if !strings.Contains(string(b), "'=1+1 formula") || !strings.Contains(string(b), "'-minus") || !strings.Contains(string(b), "'@mention") {
		t.Fatalf("formula cells not prefixed:\n%s", b)
	}
	r := doImport(t, e, e.b, "export.csv", b)
	if r.Created != 24 || r.Skipped != 0 {
		t.Fatalf("import result %+v", r)
	}
	got := fetchAll(t, e.b)
	for tag, k := range want {
		if got[tag] != k {
			t.Errorf("tag %s:\n got  %s\n want %s", tag, got[tag], k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d assets, want %d", len(got), len(want))
	}
}

func TestXLSXHeadersAndRoundTrip(t *testing.T) {
	e := setup(t)
	seedRich(t, e)
	res, b := get(t, e.a, "format=xlsx&sort=asset_tag&dir=asc")
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("status %d type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || strings.Join(rows[0], ",") != strings.Join(importer.CanonicalHeaders, ",") {
		t.Fatalf("rows=%d header=%v", len(rows), rows[0])
	}
	if rows[2][15] != "=1+1 formula" {
		t.Errorf("xlsx text must not be prefixed, got %q", rows[2][15])
	}
	if c, _ := f.GetCellType(f.GetSheetName(0), "N2"); c == excelize.CellTypeSharedString || c == excelize.CellTypeInlineString {
		t.Errorf("cost cell is text (type %v)", c)
	}
	want := fetchAll(t, e.a)
	r := doImport(t, e, e.b, "export.xlsx", b)
	if r.Created != 4 {
		t.Fatalf("import %+v", r)
	}
	got := fetchAll(t, e.b)
	for tag, k := range want {
		if got[tag] != k {
			t.Errorf("tag %s:\n got  %s\n want %s", tag, got[tag], k)
		}
	}
}

func TestPDFValid(t *testing.T) {
	e := setup(t)
	long := strings.Repeat("Péré & \"Zoë\" ", 20)
	for i := 0; i < 60; i++ {
		create(t, e.a, map[string]any{"name": fmt.Sprintf("%s %d", long, i), "category": "vehicle", "status": "in_use"})
	}
	res, b := get(t, e.a, "format=pdf&status=in_use&category=vehicle")
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("status %d", res.StatusCode)
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatal("not a pdf")
	}
	if n := bytes.Count(b, []byte("/Type /Page\n")) + bytes.Count(b, []byte("/Type /Page ")); n < 2 {
		t.Errorf("expected multiple pages, found %d", n)
	}
	if !strings.HasSuffix(res.Header.Get("Content-Disposition"), ".pdf\"") {
		t.Errorf("disposition %q", res.Header.Get("Content-Disposition"))
	}
	if dir := os.Getenv("EXPORT_PDF_OUT"); dir != "" {
		_ = os.WriteFile(dir, b, 0o644)
	}
}

func TestExportCrossTenantExcluded(t *testing.T) {
	e := setup(t)
	create(t, e.a, map[string]any{"name": "Secret A", "category": "other", "status": "available"})
	create(t, e.b, map[string]any{"name": "Mine B", "category": "other", "status": "available"})
	for _, f := range []string{"csv", "xlsx", "pdf"} {
		_, b := get(t, e.b, "format="+f)
		if f == "csv" {
			if recs := readCSV(t, b); len(recs) != 2 || recs[1][1] != "Mine B" {
				t.Fatalf("csv: %v", recs)
			}
		}
		if bytes.Contains(b, []byte("Secret A")) {
			t.Errorf("%s leaks tenant A data", f)
		}
	}
	// a location id from another tenant filters to nothing
	var loc struct{ ID string }
	res := e.a.Do("POST", "/api/locations", map[string]any{"name": "HQ"})
	e.a.JSON(res, &loc)
	_, b := get(t, e.b, "format=csv&location_id="+loc.ID)
	if recs := readCSV(t, b); len(recs) != 1 {
		t.Fatalf("foreign location filter returned rows: %v", recs)
	}
}
