package asset_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/masterdata"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type errResp struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

type ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type assetJSON struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	AssetTag        string  `json:"asset_tag"`
	Category        string  `json:"category"`
	Model           string  `json:"model"`
	Status          string  `json:"status"`
	AssignedTo      string  `json:"assigned_to"`
	LocationID      *string `json:"location_id"`
	Location        *ref    `json:"location"`
	Organization    *ref    `json:"organization"`
	Supplier        *ref    `json:"supplier"`
	PurchaseDate    *string `json:"purchase_date"`
	PurchaseCost    any     `json:"purchase_cost"`
	WarrantyMonths  *int    `json:"warranty_months"`
	WarrantyExpires *string `json:"warranty_expires"`
	WarrantyExpired bool    `json:"warranty_expired"`
	CreatedAt       string  `json:"created_at"`
}

type historyPage struct {
	Items []struct {
		ID        int64          `json:"id"`
		Event     string         `json:"event"`
		ActorName string         `json:"actor_name"`
		Detail    map[string]any `json:"detail"`
		CreatedAt string         `json:"created_at"`
	} `json:"items"`
	NextBefore *int64 `json:"next_before"`
}

type env struct {
	pool             *pgxpool.Pool
	tenantA, tenantB uuid.UUID
	a, b             *testutil.APIClient
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	md := masterdata.NewService(pool, as)
	assets := asset.NewService(pool, as)
	hist := history.NewService(pool, as)
	prints := asset.NewPrintService(pool, as, assets, "http://app.test")
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		md.Register(m)
		assets.Register(m)
		hist.Register(m)
		prints.Register(m)
	}}))
	t.Cleanup(srv.Close)
	e := &env{pool: pool}
	e.tenantA = testutil.CreateTenant(t, pool, "A")
	e.tenantB = testutil.CreateTenant(t, pool, "B")
	testutil.CreateUser(t, pool, e.tenantA, "a@a.test", "password-1")
	testutil.CreateUser(t, pool, e.tenantB, "b@b.test", "password-1")
	e.a = testutil.Client(t, srv.URL)
	e.b = testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{e.a: "a@a.test", e.b: "b@b.test"} {
		res := c.Login(em, "password-1")
		if res.StatusCode != 200 {
			t.Fatalf("login %d", res.StatusCode)
		}
		res.Body.Close()
	}
	return e
}

func baseBody(name string) map[string]any {
	return map[string]any{"name": name, "category": "it_equipment", "status": "available"}
}

// create posts body and requires 201.
func create(t *testing.T, c *testutil.APIClient, body map[string]any) assetJSON {
	t.Helper()
	res := c.Do("POST", "/api/assets", body)
	if res.StatusCode != 201 {
		var er errResp
		c.JSON(res, &er)
		t.Fatalf("create asset %d %+v", res.StatusCode, er)
	}
	var a assetJSON
	c.JSON(res, &a)
	return a
}

func expect422(t *testing.T, c *testutil.APIClient, res *http.Response, want map[string]string) {
	t.Helper()
	if res.StatusCode != 422 {
		res.Body.Close()
		t.Fatalf("status %d, want 422", res.StatusCode)
	}
	var er errResp
	c.JSON(res, &er)
	if er.Error.Code != "validation" {
		t.Fatalf("code %q", er.Error.Code)
	}
	for k, v := range want {
		if er.Error.Fields[k] != v {
			t.Fatalf("fields %+v, want %s=%s", er.Error.Fields, k, v)
		}
	}
}

func masterID(t *testing.T, c *testutil.APIClient, kind, name string) string {
	t.Helper()
	res := c.Do("POST", "/api/"+kind, map[string]string{"name": name})
	if res.StatusCode != 201 {
		t.Fatalf("create %s %d", kind, res.StatusCode)
	}
	var it map[string]any
	c.JSON(res, &it)
	return it["id"].(string)
}

func getHistory(t *testing.T, c *testutil.APIClient, id string) historyPage {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+id+"/history", nil)
	if res.StatusCode != 200 {
		res.Body.Close()
		t.Fatalf("history %d", res.StatusCode)
	}
	var p historyPage
	c.JSON(res, &p)
	return p
}

func TestCreateAutoTag(t *testing.T) {
	e := setup(t)
	body := baseBody("  Laptop Dell ")
	body["purchase_cost"] = "12500000"
	body["purchase_date"] = "2026-01-15"
	body["warranty_months"] = 12
	a := create(t, e.a, body)
	if !regexp.MustCompile(`^AST-[A-Z0-9]{8}$`).MatchString(a.AssetTag) {
		t.Fatalf("tag %q", a.AssetTag)
	}
	if a.ID == "" || a.Name != "Laptop Dell" || a.Category != "it_equipment" || a.Status != "available" {
		t.Fatalf("asset %+v", a)
	}
	if a.PurchaseCost != "12500000.00" {
		t.Fatalf("purchase_cost %#v", a.PurchaseCost)
	}
	if a.WarrantyExpires == nil || *a.WarrantyExpires != "2027-01-15" {
		t.Fatalf("warranty_expires %v", a.WarrantyExpires)
	}

	// The tag is kept when given, and GET returns the same asset.
	b := create(t, e.a, map[string]any{"name": "Kursi", "category": "furniture", "status": "in_use", "asset_tag": "  KRS-01 "})
	if b.AssetTag != "KRS-01" {
		t.Fatalf("given tag %q", b.AssetTag)
	}
	res := e.a.Do("GET", "/api/assets/"+a.ID, nil)
	var got assetJSON
	e.a.JSON(res, &got)
	if res.StatusCode != 200 || got.AssetTag != a.AssetTag || got.PurchaseCost != "12500000.00" {
		t.Fatalf("get %d %+v", res.StatusCode, got)
	}

	h := getHistory(t, e.a, a.ID)
	if len(h.Items) != 1 || h.Items[0].Event != "asset_created" || h.Items[0].ActorName != "User a@a.test" {
		t.Fatalf("history %+v", h)
	}
}

func TestCreateRequiredFields422(t *testing.T) {
	e := setup(t)
	res := e.a.Do("POST", "/api/assets", map[string]any{"name": "  "})
	expect422(t, e.a, res, map[string]string{"name": "required", "category": "required", "status": "required"})

	res = e.a.Do("POST", "/api/assets", map[string]any{
		"name": "X", "category": "spaceship", "status": "flying", "purchase_cost": "-1", "warranty_months": -3})
	expect422(t, e.a, res, map[string]string{"category": "invalid", "status": "invalid", "purchase_cost": "invalid", "warranty_months": "invalid"})

	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM assets").Scan(&n); err != nil || n != 0 {
		t.Fatalf("assets created: %d %v", n, err)
	}
}

func TestTagUniqueCaseInsensitive(t *testing.T) {
	e := setup(t)
	body := baseBody("One")
	body["asset_tag"] = "AST-001"
	create(t, e.a, body)

	body["asset_tag"] = "ast-001"
	res := e.a.Do("POST", "/api/assets", body)
	expect422(t, e.a, res, map[string]string{"asset_tag": "taken"})

	// Another tenant may use the same tag.
	create(t, e.b, body)

	// Updating another asset to a taken tag is rejected too.
	other := create(t, e.a, baseBody("Two"))
	upd := baseBody("Two")
	upd["asset_tag"] = "Ast-001"
	res = e.a.Do("PUT", "/api/assets/"+other.ID, upd)
	expect422(t, e.a, res, map[string]string{"asset_tag": "taken"})
}

func TestFKFromOtherTenant422(t *testing.T) {
	e := setup(t)
	locB := masterID(t, e.b, "locations", "Gudang B")
	orgB := masterID(t, e.b, "organizations", "Org B")
	supB := masterID(t, e.b, "suppliers", "Sup B")

	body := baseBody("X")
	body["location_id"] = locB
	body["organization_id"] = orgB
	body["supplier_id"] = supB
	res := e.a.Do("POST", "/api/assets", body)
	expect422(t, e.a, res, map[string]string{"location_id": "invalid", "organization_id": "invalid", "supplier_id": "invalid"})

	body = baseBody("X")
	body["location_id"] = uuid.NewString()
	res = e.a.Do("POST", "/api/assets", body)
	expect422(t, e.a, res, map[string]string{"location_id": "invalid"})

	// Own master data works and comes back with names.
	locA := masterID(t, e.a, "locations", "Gudang A")
	body = baseBody("X")
	body["location_id"] = locA
	a := create(t, e.a, body)
	if a.Location == nil || a.Location.ID != locA || a.Location.Name != "Gudang A" || a.Organization != nil {
		t.Fatalf("refs %+v %+v", a.Location, a.Organization)
	}

	// Update to another tenant's location is rejected as well.
	body["location_id"] = locB
	res = e.a.Do("PUT", "/api/assets/"+a.ID, body)
	expect422(t, e.a, res, map[string]string{"location_id": "invalid"})
}

func TestUpdateRecordsHistory(t *testing.T) {
	e := setup(t)
	loc1 := masterID(t, e.a, "locations", "Gudang 1")
	loc2 := masterID(t, e.a, "locations", "Gudang 2")
	body := baseBody("Laptop")
	body["location_id"] = loc1
	body["assigned_to"] = "Budi"
	body["model"] = "X1"
	a := create(t, e.a, body)

	body["status"] = "in_use"
	body["location_id"] = loc2
	body["assigned_to"] = "Siti"
	body["model"] = "X2"
	body["asset_tag"] = a.AssetTag
	res := e.a.Do("PUT", "/api/assets/"+a.ID, body)
	if res.StatusCode != 200 {
		res.Body.Close()
		t.Fatalf("put %d", res.StatusCode)
	}
	var got assetJSON
	e.a.JSON(res, &got)
	if got.Status != "in_use" || got.Model != "X2" || got.AssignedTo != "Siti" || got.Location == nil || got.Location.Name != "Gudang 2" {
		t.Fatalf("updated %+v", got)
	}

	h := getHistory(t, e.a, a.ID)
	if len(h.Items) != 5 {
		t.Fatalf("want 4 update entries + created, got %+v", h.Items)
	}
	byEvent := map[string]map[string]any{}
	for _, it := range h.Items[:4] {
		byEvent[it.Event] = it.Detail
	}
	if d := byEvent["status_changed"]; d == nil || d["from"] != "available" || d["to"] != "in_use" {
		t.Fatalf("status_changed %+v", byEvent)
	}
	if d := byEvent["location_changed"]; d == nil || d["from"] != "Gudang 1" || d["to"] != "Gudang 2" {
		t.Fatalf("location_changed %+v", byEvent)
	}
	if d := byEvent["assignee_changed"]; d == nil || d["from"] != "Budi" || d["to"] != "Siti" {
		t.Fatalf("assignee_changed %+v", byEvent)
	}
	d := byEvent["details_updated"]
	if d == nil || fmt.Sprint(d["fields"]) != "[model]" {
		t.Fatalf("details_updated %+v", byEvent)
	}
	if h.Items[4].Event != "asset_created" {
		t.Fatalf("oldest %+v", h.Items[4])
	}
}

func TestUpdateNoChangeNoHistory(t *testing.T) {
	e := setup(t)
	body := baseBody("Laptop")
	body["purchase_cost"] = "100.5"
	body["purchase_date"] = "2026-02-01"
	body["warranty_months"] = 6
	a := create(t, e.a, body)
	body["asset_tag"] = a.AssetTag
	body["purchase_cost"] = "100.50"
	body["name"] = " Laptop "
	res := e.a.Do("PUT", "/api/assets/"+a.ID, body)
	if res.StatusCode != 200 {
		res.Body.Close()
		t.Fatalf("put %d", res.StatusCode)
	}
	res.Body.Close()
	h := getHistory(t, e.a, a.ID)
	if len(h.Items) != 1 || h.Items[0].Event != "asset_created" {
		t.Fatalf("history %+v", h.Items)
	}
}

func TestCrossTenantGetPutDelete404(t *testing.T) {
	e := setup(t)
	b := create(t, e.b, baseBody("Milik B"))

	for _, c := range []struct {
		method string
		body   any
	}{{"GET", nil}, {"PUT", baseBody("Dibajak")}, {"DELETE", nil}} {
		res := e.a.Do(c.method, "/api/assets/"+b.ID, c.body)
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatalf("%s other tenant: %d", c.method, res.StatusCode)
		}
	}
	res := e.a.Do("GET", "/api/assets/not-a-uuid", nil)
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("bad id %d", res.StatusCode)
	}

	res = e.b.Do("GET", "/api/assets/"+b.ID, nil)
	var got assetJSON
	e.b.JSON(res, &got)
	if res.StatusCode != 200 || got.Name != "Milik B" {
		t.Fatalf("B's asset changed: %d %+v", res.StatusCode, got)
	}
	if h := getHistory(t, e.b, b.ID); len(h.Items) != 1 {
		t.Fatalf("B history %+v", h.Items)
	}
}

func TestDeleteCascadesHistory(t *testing.T) {
	e := setup(t)
	a := create(t, e.a, baseBody("Hapus"))
	res := e.a.Do("DELETE", "/api/assets/"+a.ID, nil)
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("delete %d", res.StatusCode)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM asset_history WHERE asset_id = $1", a.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("history rows left: %d %v", n, err)
	}
	res = e.a.Do("GET", "/api/assets/"+a.ID, nil)
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("get after delete %d", res.StatusCode)
	}
	res = e.a.Do("DELETE", "/api/assets/"+a.ID, nil)
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("delete again %d", res.StatusCode)
	}
}

func TestMasterDeleteInUse409(t *testing.T) {
	e := setup(t)
	loc := masterID(t, e.a, "locations", "Gudang")
	masterID(t, e.a, "locations", "Kosong")
	for _, n := range []string{"A1", "A2"} {
		body := baseBody(n)
		body["location_id"] = loc
		create(t, e.a, body)
	}
	var list []map[string]any
	res := e.a.Do("GET", "/api/locations", nil)
	e.a.JSON(res, &list)
	counts := map[string]any{}
	for _, it := range list {
		counts[it["name"].(string)] = it["asset_count"]
	}
	if counts["Gudang"] != float64(2) || counts["Kosong"] != float64(0) {
		t.Fatalf("asset_count %+v", counts)
	}

	res = e.a.Do("DELETE", "/api/locations/"+loc, nil)
	if res.StatusCode != 409 {
		res.Body.Close()
		t.Fatalf("delete in use %d", res.StatusCode)
	}
	var er errResp
	e.a.JSON(res, &er)
	if er.Error.Code != "in_use" || er.Error.Fields["count"] != "2" {
		t.Fatalf("%+v", er)
	}
}

func TestAssigneeSuggestions(t *testing.T) {
	e := setup(t)
	for _, n := range []string{"Budi Santoso", "Budi Santoso", "Pak budiman", "Ani", ""} {
		body := baseBody("X")
		body["assigned_to"] = n
		create(t, e.a, body)
	}
	body := baseBody("X")
	body["assigned_to"] = "Budi Lain Tenant"
	create(t, e.b, body)

	var got []string
	res := e.a.Do("GET", "/api/assets/assignees?q=BUDI", nil)
	e.a.JSON(res, &got)
	if res.StatusCode != 200 || !slices.Equal(got, []string{"Budi Santoso", "Pak budiman"}) {
		t.Fatalf("suggestions %d %q", res.StatusCode, got)
	}

	// A literal % is not a wildcard; at most 10 names are returned.
	res = e.a.Do("GET", "/api/assets/assignees?q=%25", nil)
	e.a.JSON(res, &got)
	if len(got) != 0 {
		t.Fatalf("wildcard %q", got)
	}
	for i := range 12 {
		body := baseBody("X")
		body["assigned_to"] = fmt.Sprintf("Staff %02d", i)
		create(t, e.a, body)
	}
	res = e.a.Do("GET", "/api/assets/assignees", nil)
	e.a.JSON(res, &got)
	if len(got) != 10 || got[0] != "Budi Santoso" {
		t.Fatalf("limit %q", got)
	}
}

func TestWarrantyExpiredFlag(t *testing.T) {
	e := setup(t)
	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(jkt)

	body := baseBody("Lama")
	body["purchase_date"] = "2020-01-31"
	body["warranty_months"] = 1
	old := create(t, e.a, body)
	if old.WarrantyExpires == nil || *old.WarrantyExpires != "2020-02-29" || !old.WarrantyExpired {
		t.Fatalf("old %v %v", old.WarrantyExpires, old.WarrantyExpired)
	}

	body = baseBody("Baru")
	body["purchase_date"] = today.Format("2006-01-02")
	body["warranty_months"] = 12
	fresh := create(t, e.a, body)
	if fresh.WarrantyExpires == nil || fresh.WarrantyExpired {
		t.Fatalf("fresh %v %v", fresh.WarrantyExpires, fresh.WarrantyExpired)
	}

	// Expiring today is not yet expired.
	body = baseBody("Hari ini")
	body["purchase_date"] = today.Format("2006-01-02")
	body["warranty_months"] = 0
	if a := create(t, e.a, body); a.WarrantyExpired {
		t.Fatalf("expires today flagged expired")
	}

	body = baseBody("Tanpa garansi")
	body["purchase_date"] = "2020-01-01"
	none := create(t, e.a, body)
	if none.WarrantyExpires != nil || none.WarrantyExpired {
		t.Fatalf("none %v %v", none.WarrantyExpires, none.WarrantyExpired)
	}
}

// The tx-level core used by the importer: explicit event, one entry listing
// every changed field, and the changed list returned to the caller.
func TestTxCoreImportEvents(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	svc := asset.NewService(e.pool, auth.NewService(e.pool, false))
	p := auth.Principal{TenantID: e.tenantA, Name: "Importer"}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) // no-op after Commit; releases the conn if a check fails
	a, err := svc.CreateTx(ctx, tx, p, asset.Input{Name: "Imp", AssetTag: "IMP-1", Category: "other", Status: "available"}, history.EventAssetImportedCreated)
	if err != nil {
		t.Fatal(err)
	}
	// A taken tag inside a tx reports the validation error without aborting the tx.
	_, err = svc.CreateTx(ctx, tx, p, asset.Input{Name: "Dup", AssetTag: "imp-1", Category: "other", Status: "available"}, history.EventAssetImportedCreated)
	var ve *asset.ValidationError
	if !errors.As(err, &ve) || ve.Fields["asset_tag"] != "taken" {
		t.Fatalf("dup tag: %v", err)
	}
	in := a.Input
	in.Status = "in_use"
	in.Notes = "baru"
	_, changed, err := svc.UpdateTx(ctx, tx, p, a.ID, in, history.EventAssetImportedUpdated)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{"status", "notes"}) {
		t.Fatalf("changed %v", changed)
	}
	_, changed, err = svc.UpdateTx(ctx, tx, p, a.ID, in, history.EventAssetImportedUpdated)
	if err != nil || len(changed) != 0 {
		t.Fatalf("unchanged: %v %v", changed, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	rows, err := e.pool.Query(ctx, "SELECT event, detail::text, actor_id IS NULL FROM asset_history WHERE asset_id = $1 ORDER BY id", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var ev, detail string
		var nullActor bool
		if err := rows.Scan(&ev, &detail, &nullActor); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s %v", ev, detail, nullActor))
	}
	want := []string{"asset_imported_created {} true", `asset_imported_updated {"fields": ["status", "notes"]} true`}
	if !slices.Equal(got, want) {
		t.Fatalf("history %q", got)
	}

	if _, err := svc.Get(ctx, e.tenantB, a.ID); !errors.Is(err, asset.ErrNotFound) {
		t.Fatalf("cross-tenant service get: %v", err)
	}
}

// GetAsset must not leak another tenant's master-data name even when a row
// points at it (the FK alone does not enforce the tenant).
func TestGetAssetIgnoresForeignTenantMasterData(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	a := testutil.CreateTenant(t, pool, "A")
	b := testutil.CreateTenant(t, pool, "B")
	var loc, org, sup, id uuid.UUID
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO locations (tenant_id, name) VALUES ($1, 'B-Loc') RETURNING id`, b).Scan(&loc))
	must(pool.QueryRow(ctx, `INSERT INTO organizations (tenant_id, name) VALUES ($1, 'B-Org') RETURNING id`, b).Scan(&org))
	must(pool.QueryRow(ctx, `INSERT INTO suppliers (tenant_id, name) VALUES ($1, 'B-Sup') RETURNING id`, b).Scan(&sup))
	must(pool.QueryRow(ctx, `INSERT INTO assets (tenant_id, name, asset_tag, category, status, location_id, organization_id, supplier_id)
		VALUES ($1, 'X', 'AST-XTENANT', 'other', 'available', $2, $3, $4) RETURNING id`, a, loc, org, sup).Scan(&id))
	r, err := gen.New(pool).GetAsset(ctx, gen.GetAssetParams{TenantID: a, ID: id})
	must(err)
	if r.LocationName != nil || r.OrganizationName != nil || r.SupplierName != nil {
		t.Fatalf("leaked names: %v %v %v", r.LocationName, r.OrganizationName, r.SupplierName)
	}
}
