package importer

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/masterdata"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

func setupTenant(t *testing.T) (*pgxpool.Pool, auth.Principal) {
	t.Helper()
	pool := testutil.NewDB(t)
	tenant := testutil.CreateTenant(t, pool, "T")
	user := testutil.CreateUser(t, pool, tenant, "u@t.test", "password-1")
	return pool, auth.Principal{UserID: user, TenantID: tenant, Name: "Importer"}
}

func importFile(t *testing.T, pool *pgxpool.Pool, p auth.Principal, name, content string) *Result {
	t.Helper()
	f, err := parseFile(name, strings.NewReader(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := process(context.Background(), pool, p, f, nil)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	return res
}

func counts(r *Result) [4]int { return [4]int{r.Created, r.Updated, r.Unchanged, r.Skipped} }

func findTag(t *testing.T, pool *pgxpool.Pool, p auth.Principal, tag string) asset.Asset {
	t.Helper()
	a, ok, err := asset.FindByTag(context.Background(), pool, p.TenantID, tag)
	if err != nil || !ok {
		t.Fatalf("find %s: ok=%v err=%v", tag, ok, err)
	}
	return a
}

func assetCount(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM assets WHERE tenant_id = $1`, tenant).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const threeRows = `Asset Tag,Asset Name,Category,Status,Location,Supplier,Purchase Date,Purchase Cost,Warranty Months
AST-A1,Laptop,Laptop,Tersedia,Gudang,Vendor X,15/01/2026,Rp 12.500.000,24
AST-A2,Mobil,Kendaraan,Dipakai,Gudang,Vendor X,2026-02-01,"250,000,000.00",1 tahun
AST-A3,Meja,Furniture,Rusak,Kantor,,45678,1500000.5,12 bulan
`

func TestImportCreatesUpdatesUnchanged(t *testing.T) {
	pool, p := setupTenant(t)
	if got := counts(importFile(t, pool, p, "a.csv", threeRows)); got != [4]int{3, 0, 0, 0} {
		t.Fatalf("first run counts=%v", got)
	}
	a1 := findTag(t, pool, p, "AST-A1")
	if a1.Category != "it_equipment" || a1.Status != "available" || a1.PurchaseCost.String() != "12500000" ||
		a1.PurchaseDate.String() != "2026-01-15" || *a1.WarrantyMonths != 24 || a1.Location.Name != "Gudang" {
		t.Errorf("a1=%+v", a1)
	}
	if a3 := findTag(t, pool, p, "AST-A3"); a3.PurchaseCost.StringFixed(2) != "1500000.50" || a3.Status != "damaged" {
		t.Errorf("a3 cost=%v status=%s", a3.PurchaseCost, a3.Status)
	}
	if got := counts(importFile(t, pool, p, "a.csv", threeRows)); got != [4]int{0, 0, 3, 0} {
		t.Fatalf("second run counts=%v", got)
	}
	changed := strings.Replace(threeRows, "Mobil,", "Mobil Dinas,", 1)
	if got := counts(importFile(t, pool, p, "a.csv", changed)); got != [4]int{0, 1, 2, 0} {
		t.Fatalf("third run counts=%v", got)
	}
	a2 := findTag(t, pool, p, "AST-A2")
	var events []string
	rows, err := pool.Query(context.Background(),
		`SELECT event || ':' || detail::text FROM asset_history WHERE asset_id = $1 ORDER BY id`, a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		events = append(events, s)
	}
	want := []string{`asset_imported_created:{}`, `asset_imported_updated:{"fields": ["name"]}`}
	if strings.Join(events, "|") != strings.Join(want, "|") {
		t.Errorf("history=%q", events)
	}
}

func TestEmptyCellDoesNotOverwrite(t *testing.T) {
	pool, p := setupTenant(t)
	importFile(t, pool, p, "a.csv", "Asset Tag,Name,Manufacturer,Model,Notes,Purchase Cost\nT-1,Laptop,Dell,E5,keep me,100\n")
	res := importFile(t, pool, p, "a.csv", "Asset Tag,Name,Manufacturer,Model,Notes,Purchase Cost\nT-1,Laptop,,E6,,\n")
	if got := counts(res); got != [4]int{0, 1, 0, 0} {
		t.Fatalf("counts=%v", got)
	}
	a := findTag(t, pool, p, "T-1")
	if a.Manufacturer != "Dell" || a.Model != "E6" || a.Notes != "keep me" || a.PurchaseCost.String() != "100" {
		t.Errorf("asset=%+v", a.Input)
	}
}

func TestTagCaseInsensitiveUpdate(t *testing.T) {
	pool, p := setupTenant(t)
	if _, err := asset.NewService(pool, nil).Create(context.Background(), p,
		asset.Input{Name: "Old", AssetTag: "AST-001", Category: "other", Status: "available"}); err != nil {
		t.Fatal(err)
	}
	res := importFile(t, pool, p, "a.csv", "Asset Tag,Name\nast-001,New name\n")
	if got := counts(res); got != [4]int{0, 1, 0, 0} {
		t.Fatalf("counts=%v", got)
	}
	if a := findTag(t, pool, p, "AST-001"); a.AssetTag != "AST-001" || a.Name != "New name" {
		t.Errorf("asset tag=%s name=%s", a.AssetTag, a.Name)
	}
	if n := assetCount(t, pool, p.TenantID); n != 1 {
		t.Errorf("asset count=%d", n)
	}
}

func TestDuplicateTagInFileSkipped(t *testing.T) {
	pool, p := setupTenant(t)
	res := importFile(t, pool, p, "a.csv", "Tag,Name\nD-1,One\nd-1,Two\n,Three\n,Four\n")
	if got := counts(res); got != [4]int{3, 0, 0, 1} {
		t.Fatalf("counts=%v", got)
	}
	if len(res.SkippedRows) != 1 || res.SkippedRows[0] != (RowIssue{Row: 3, Reason: ReasonDuplicateTag}) {
		t.Errorf("skipped=%+v", res.SkippedRows)
	}
	if a := findTag(t, pool, p, "D-1"); a.Name != "One" {
		t.Errorf("name=%s", a.Name)
	}
}

func TestMissingNameSkippedAndBlankRowsIgnored(t *testing.T) {
	pool, p := setupTenant(t)
	res := importFile(t, pool, p, "a.csv", "Daftar Aset\n\nName,Tag,Notes\nA,,\n,,\n , , \n,X-9,orphan\nB,,\n")
	if got := counts(res); got != [4]int{2, 0, 0, 1} {
		t.Fatalf("counts=%v", got)
	}
	// title line 1, blank line 2, header line 3, data lines 4..8: the nameless row is line 7
	if len(res.SkippedRows) != 1 || res.SkippedRows[0].Reason != ReasonMissingName || res.SkippedRows[0].Row != 7 {
		t.Errorf("skipped=%+v", res.SkippedRows)
	}
}

func TestUnknownOrganizationBlankedAndReported(t *testing.T) {
	pool, p := setupTenant(t)
	md := masterdata.NewService(pool, nil)
	ctx := context.Background()
	satu, err := md.Create(ctx, p.TenantID, masterdata.Organizations, map[string]string{"name": "PT Satu"})
	if err != nil {
		t.Fatal(err)
	}
	res := importFile(t, pool, p, "a.csv",
		"Tag,Name,Company\nO-1,A, pt satu \nO-2,B,PT Dua\nO-3,C,pt dua\nO-4,D,PT Tiga\n")
	want := []NameCount{{"PT Dua", 2}, {"PT Tiga", 1}}
	if len(res.UnmatchedOrganizations) != 2 || res.UnmatchedOrganizations[0] != want[0] || res.UnmatchedOrganizations[1] != want[1] {
		t.Errorf("unmatched=%+v", res.UnmatchedOrganizations)
	}
	if a := findTag(t, pool, p, "O-1"); a.Organization == nil || a.Organization.ID != satu["id"] {
		t.Errorf("O-1 org=%+v", a.Organization)
	}
	if a := findTag(t, pool, p, "O-2"); a.Organization != nil {
		t.Errorf("O-2 org=%+v", a.Organization)
	}
	// An unmatched name never blanks an existing organization.
	res = importFile(t, pool, p, "a.csv", "Tag,Name,Company\nO-1,A,PT Empat\n")
	if got := counts(res); got != [4]int{0, 0, 1, 0} {
		t.Errorf("counts=%v", got)
	}
	if a := findTag(t, pool, p, "O-1"); a.Organization == nil {
		t.Error("existing organization was blanked")
	}
}

func TestSupplierAndLocationAutoCreated(t *testing.T) {
	pool, p := setupTenant(t)
	md := masterdata.NewService(pool, nil)
	if _, err := md.Create(context.Background(), p.TenantID, masterdata.Locations, map[string]string{"name": "Gudang"}); err != nil {
		t.Fatal(err)
	}
	res := importFile(t, pool, p, "a.csv",
		"Name,Vendor,Lokasi\nA,Vendor X,gudang\nB,vendor x,Kantor\nC,,Kantor\n")
	if strings.Join(res.CreatedSuppliers, "|") != "Vendor X" || strings.Join(res.CreatedLocations, "|") != "Kantor" {
		t.Errorf("suppliers=%q locations=%q", res.CreatedSuppliers, res.CreatedLocations)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM suppliers WHERE tenant_id = $1`, p.TenantID).Scan(&n)
	if n != 1 {
		t.Errorf("suppliers=%d", n)
	}
}

func TestUnknownStatusBecomesAvailableAndReported(t *testing.T) {
	pool, p := setupTenant(t)
	importFile(t, pool, p, "a.csv", "Tag,Name,Status,Category\nS-1,A,Dipakai,Laptop\n")
	res := importFile(t, pool, p, "a.csv",
		"Tag,Name,Status,Category\nS-1,A,Melayang,Spaceship\nS-2,B,melayang,\nS-3,C,Ready to Deploy,Laptop\n")
	if got := counts(res); got != [4]int{2, 0, 1, 0} {
		t.Fatalf("counts=%v", got)
	}
	if a := findTag(t, pool, p, "S-1"); a.Status != "in_use" || a.Category != "it_equipment" {
		t.Errorf("existing overwritten: status=%s category=%s", a.Status, a.Category)
	}
	if a := findTag(t, pool, p, "S-2"); a.Status != "available" || a.Category != "other" {
		t.Errorf("new: status=%s category=%s", a.Status, a.Category)
	}
	want := map[ValueIssue]bool{
		{"status", "Melayang", 2}:    true,
		{"category", "Spaceship", 1}: true,
		{"category", "", 1}:          true,
	}
	if len(res.UnrecognizedValues) != len(want) {
		t.Fatalf("unrecognized=%+v", res.UnrecognizedValues)
	}
	for _, v := range res.UnrecognizedValues {
		if !want[v] {
			t.Errorf("unexpected %+v", v)
		}
	}
}

func TestSnipeITExport(t *testing.T) {
	pool, p := setupTenant(t)
	csv := "Asset Tag,Asset Name,Category,Manufacturer,Model,Serial,Status,Location,Checked Out To,Company,Supplier,Order Number,Purchase Date,Purchase Cost,Warranty,Notes\n" +
		"SN-1,MacBook,Laptops,Apple,M3,C02X,Ready to Deploy,HQ,,,,PO-1,2025-03-01,2000.00,12 months,\n" +
		"SN-2,ThinkPad,Laptops,Lenovo,T14,PF1,Deployed,HQ,Siti Aminah,,,,,,,\n"
	res := importFile(t, pool, p, "export.csv", csv)
	if got := counts(res); got != [4]int{2, 0, 0, 0} || len(res.UnknownColumns) != 0 {
		t.Fatalf("counts=%v unknown=%q", got, res.UnknownColumns)
	}
	a1, a2 := findTag(t, pool, p, "SN-1"), findTag(t, pool, p, "SN-2")
	if a1.Status != "available" || a1.SerialNumber != "C02X" || *a1.WarrantyMonths != 12 || a1.OrderNumber != "PO-1" {
		t.Errorf("a1=%+v", a1.Input)
	}
	if a2.Status != "in_use" || a2.AssignedTo != "Siti Aminah" {
		t.Errorf("a2 status=%s assigned=%q", a2.Status, a2.AssignedTo)
	}
}

func TestMessyIndonesianSpreadsheet(t *testing.T) {
	pool, p := setupTenant(t)
	csv := "\xef\xbb\xbfDaftar Inventaris Kantor 2026;;;;;\r\n" +
		" Nama Asset ;KODE_ASET;Kategori;Lokasi;Harga Pembelian;Tanggal Beli\r\n" +
		" Laptop Kantor ; INV-01 ;Komputer; Ruang Rapat ;Rp 12.500.000;11/11/2026\r\n"
	res := importFile(t, pool, p, "Inventaris.CSV", csv)
	if got := counts(res); got != [4]int{1, 0, 0, 0} {
		t.Fatalf("counts=%v skipped=%+v", got, res.SkippedRows)
	}
	a := findTag(t, pool, p, "INV-01")
	if a.Name != "Laptop Kantor" || a.Category != "it_equipment" || a.Location == nil || a.Location.Name != "Ruang Rapat" ||
		a.PurchaseCost.String() != "12500000" || a.PurchaseDate.String() != "2026-11-11" {
		t.Errorf("asset=%+v location=%+v", a.Input, a.Location)
	}
}

func TestInvalidValuesSkipped(t *testing.T) {
	pool, p := setupTenant(t)
	res := importFile(t, pool, p, "a.csv", "Name,Purchase Date,Purchase Cost,Warranty\n"+
		"A,31/02/2026,,\nB,,abc,\nC,,,forever\nD,,100000000000000,\nE,,,5000\nF,,,\n")
	if got := counts(res); got != [4]int{1, 0, 0, 5} {
		t.Fatalf("counts=%v", got)
	}
	want := []string{"invalid_value:purchase_date", "invalid_value:purchase_cost", "invalid_value:warranty_months",
		"invalid_value:purchase_cost", "invalid_value:warranty_months"}
	for i, r := range res.SkippedRows {
		if r.Row != i+2 || r.Reason != want[i] {
			t.Errorf("skipped[%d]=%+v", i, r)
		}
	}
	if n := assetCount(t, pool, p.TenantID); n != 1 {
		t.Errorf("asset count=%d", n)
	}
}

func TestApplyRowRaggedRow(t *testing.T) {
	pool, p := setupTenant(t)
	res := importFile(t, pool, p, "a.csv", "Name,Tag,Notes,Model\nShort\n")
	if got := counts(res); got != [4]int{1, 0, 0, 0} {
		t.Fatalf("counts=%v", got)
	}
}

func TestTemplateRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTemplate(&buf); err != nil {
		t.Fatal(err)
	}
	f, err := parseFile("template.xlsx", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.unknown) != 0 || len(f.cols) != len(CanonicalHeaders) || len(f.rows) != 1 {
		t.Fatalf("cols=%v unknown=%q rows=%d", f.cols, f.unknown, len(f.rows))
	}
	pool, p := setupTenant(t)
	res, err := process(context.Background(), pool, p, f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := counts(res); got != [4]int{1, 0, 0, 0} || len(res.UnrecognizedValues) != 0 {
		t.Errorf("counts=%v unrecognized=%+v", got, res.UnrecognizedValues)
	}
	if len(res.CreatedSuppliers)+len(res.CreatedLocations)+len(res.UnmatchedOrganizations) != 0 {
		t.Errorf("template touched master data: %+v", res)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM locations) + (SELECT count(*) FROM suppliers)
		+ (SELECT count(*) FROM organizations)`).Scan(&n)
	if n != 0 {
		t.Errorf("master data rows=%d", n)
	}
}

func TestBlankEnumsReportedOnlyWhenColumnPresent(t *testing.T) {
	pool, p := setupTenant(t)
	res := importFile(t, pool, p, "a.csv", "Name,Notes\nA,x\nB,y\n")
	if res.Created != 2 || len(res.UnrecognizedValues) != 0 {
		t.Errorf("no enum columns: created=%d unrecognized=%+v", res.Created, res.UnrecognizedValues)
	}
	var cat, st string
	_ = pool.QueryRow(context.Background(), `SELECT category, status FROM assets WHERE name = 'A'`).Scan(&cat, &st)
	if cat != "other" || st != "available" {
		t.Errorf("fallback category=%s status=%s", cat, st)
	}
	res = importFile(t, pool, p, "a.csv", "Name,Status,Category\nC,,\nD,,Laptop\n")
	want := map[ValueIssue]bool{{"status", "", 2}: true, {"category", "", 1}: true}
	if len(res.UnrecognizedValues) != 2 || !want[res.UnrecognizedValues[0]] || !want[res.UnrecognizedValues[1]] {
		t.Errorf("present but blank: %+v", res.UnrecognizedValues)
	}
}

// TestConcurrentSameNewTag runs the same new tag in two transactions: the
// second waits on the first's unique index entry, then is skipped as "error".
func TestConcurrentSameNewTag(t *testing.T) {
	pool, p := setupTenant(t)
	ctx := context.Background()
	f, err := parseFile("a.csv", strings.NewReader("Tag,Name\nC-1,First\n"))
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows[0]
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	res1 := NewResult()
	if err := ApplyRow(ctx, tx1, p, f.cols, row.cells, row.num, res1, map[string]int{}); err != nil || res1.Created != 1 {
		t.Fatalf("first: err=%v res=%+v", err, res1)
	}
	res2 := NewResult()
	done := make(chan error, 1)
	go func() {
		done <- pgx.BeginFunc(ctx, pool, func(tx2 pgx.Tx) error {
			return ApplyRow(ctx, tx2, p, f.cols, row.cells, row.num, res2, map[string]int{})
		})
	}()
	// Commit only once the second insert is blocked on the first one's lock,
	// so the order is deterministic.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second import never blocked on the first")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := counts(res2); got != [4]int{0, 0, 0, 1} || res2.SkippedRows[0].Reason != ReasonError {
		t.Errorf("second: counts=%v skipped=%+v", got, res2.SkippedRows)
	}
	if n := assetCount(t, pool, p.TenantID); n != 1 {
		t.Errorf("asset count=%d", n)
	}
}

func TestWriteReportFormulaSafe(t *testing.T) {
	res := NewResult()
	res.skip(4, ReasonMissingName)
	res.addUnmatchedOrg("=HYPERLINK(\"x\")")
	res.addUnrecognized("status", "+cmd")
	res.addUnrecognized("category", "@SUM")
	var buf bytes.Buffer
	if err := WriteReport(&buf, res); err != nil {
		t.Fatal(err)
	}
	want := "row,reason\n4,missing_name\n\nunmatched_organization,rows\n\"'=HYPERLINK(\"\"x\"\")\",1\n\n" +
		"unrecognized_field,value,rows\nstatus,'+cmd,1\ncategory,'@SUM,1\n"
	if buf.String() != want {
		t.Errorf("report=\n%s", buf.String())
	}
}
