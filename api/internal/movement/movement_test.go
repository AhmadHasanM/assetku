package movement_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/masterdata"
	"assetku/api/internal/movement"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type errResp struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

type mv struct {
	ID               string  `json:"id"`
	FromLocationID   *string `json:"from_location_id"`
	FromLocationName string  `json:"from_location_name"`
	ToLocationID     string  `json:"to_location_id"`
	ToLocationName   string  `json:"to_location_name"`
	Date             string  `json:"date"`
	MovedBy          string  `json:"moved_by"`
	Reason           string  `json:"reason"`
}

type histPage struct {
	Items []struct {
		Event  string         `json:"event"`
		Detail map[string]any `json:"detail"`
	} `json:"items"`
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
	mvs := movement.NewService(pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		md.Register(m)
		assets.Register(m)
		hist.Register(m)
		mvs.Register(m)
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

func createLocation(t *testing.T, c *testutil.APIClient, name string) string {
	t.Helper()
	res := c.Do("POST", "/api/locations", map[string]any{"name": name})
	if res.StatusCode != 201 {
		t.Fatalf("create location %d", res.StatusCode)
	}
	var l struct {
		ID string `json:"id"`
	}
	c.JSON(res, &l)
	return l.ID
}

func createAsset(t *testing.T, c *testutil.APIClient, locationID string) string {
	t.Helper()
	body := map[string]any{"name": "Laptop", "category": "it_equipment", "status": "available"}
	if locationID != "" {
		body["location_id"] = locationID
	}
	res := c.Do("POST", "/api/assets", body)
	if res.StatusCode != 201 {
		t.Fatalf("create asset %d", res.StatusCode)
	}
	var a struct {
		ID string `json:"id"`
	}
	c.JSON(res, &a)
	return a.ID
}

type assetView struct {
	LocationID *string `json:"location_id"`
	Location   *struct {
		Name string `json:"name"`
	} `json:"location"`
	UpdatedAt time.Time `json:"updated_at"`
}

func getAsset(t *testing.T, c *testutil.APIClient, id string) assetView {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+id, nil)
	var a assetView
	c.JSON(res, &a)
	return a
}

func addMove(t *testing.T, c *testutil.APIClient, assetID string, body map[string]any) mv {
	t.Helper()
	res := c.Do("POST", "/api/assets/"+assetID+"/movements", body)
	if res.StatusCode != 201 {
		var er errResp
		c.JSON(res, &er)
		t.Fatalf("add movement %d %+v", res.StatusCode, er)
	}
	var m mv
	c.JSON(res, &m)
	return m
}

func listMoves(t *testing.T, c *testutil.APIClient, assetID string) []mv {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+assetID+"/movements", nil)
	if res.StatusCode != 200 {
		t.Fatalf("list %d", res.StatusCode)
	}
	var out struct {
		Items []mv `json:"items"`
	}
	c.JSON(res, &out)
	return out.Items
}

func hist(t *testing.T, c *testutil.APIClient, assetID string) histPage {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+assetID+"/history", nil)
	var h histPage
	c.JSON(res, &h)
	return h
}

func expect422(t *testing.T, c *testutil.APIClient, res *http.Response, field, code string) {
	t.Helper()
	var er errResp
	status := res.StatusCode
	c.JSON(res, &er)
	if status != 422 || er.Error.Code != "validation" || er.Error.Fields[field] != code {
		t.Fatalf("status %d %+v, want 422 %s=%s", status, er.Error, field, code)
	}
}

func TestMovementUpdatesLocationAndHistory(t *testing.T) {
	e := setup(t)
	gudang := createLocation(t, e.a, "Gudang")
	kantor := createLocation(t, e.a, "Kantor")
	id := createAsset(t, e.a, gudang)
	before := getAsset(t, e.a, id)

	m := addMove(t, e.a, id, map[string]any{"to_location_id": kantor, "date": "2026-05-01", "reason": "Pindah ruang"})
	if m.FromLocationName != "Gudang" || m.FromLocationID == nil || *m.FromLocationID != gudang ||
		m.ToLocationID != kantor || m.ToLocationName != "Kantor" {
		t.Fatalf("movement %+v", m)
	}
	if m.MovedBy != "User a@a.test" {
		t.Fatalf("moved_by defaulted to %q", m.MovedBy)
	}
	after := getAsset(t, e.a, id)
	if after.LocationID == nil || *after.LocationID != kantor || after.Location.Name != "Kantor" {
		t.Fatalf("asset location %+v", after)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at not bumped: %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
	h := hist(t, e.a, id)
	if h.Items[0].Event != "movement_added" || h.Items[0].Detail["from"] != "Gudang" ||
		h.Items[0].Detail["to"] != "Kantor" || h.Items[0].Detail["reason"] != "Pindah ruang" {
		t.Fatalf("history %+v", h.Items[0])
	}
	for _, it := range h.Items {
		if it.Event == "location_changed" {
			t.Fatal("unexpected separate location_changed entry")
		}
	}

	// Explicit moved_by; newest date first.
	addMove(t, e.a, id, map[string]any{"to_location_id": gudang, "date": "2026-06-01", "moved_by": "Budi"})
	items := listMoves(t, e.a, id)
	if len(items) != 2 || items[0].Date != "2026-06-01" || items[0].MovedBy != "Budi" ||
		items[0].FromLocationName != "Kantor" || items[1].Date != "2026-05-01" {
		t.Fatalf("list %+v", items)
	}
}

func TestMovementFromNoLocation(t *testing.T) {
	e := setup(t)
	kantor := createLocation(t, e.a, "Kantor")
	id := createAsset(t, e.a, "")
	m := addMove(t, e.a, id, map[string]any{"to_location_id": kantor, "date": "2026-05-01"})
	if m.FromLocationID != nil || m.FromLocationName != "" {
		t.Fatalf("from should be empty: %+v", m)
	}
	h := hist(t, e.a, id)
	if h.Items[0].Event != "movement_added" || h.Items[0].Detail["from"] != "" {
		t.Fatalf("history %+v", h.Items[0])
	}
}

func TestDeleteMovementKeepsLocation(t *testing.T) {
	e := setup(t)
	gudang := createLocation(t, e.a, "Gudang")
	kantor := createLocation(t, e.a, "Kantor")
	id := createAsset(t, e.a, gudang)
	m := addMove(t, e.a, id, map[string]any{"to_location_id": kantor, "date": "2026-05-01"})
	res := e.a.Do("DELETE", "/api/movements/"+m.ID, nil)
	if res.StatusCode != 204 {
		t.Fatalf("delete %d", res.StatusCode)
	}
	res.Body.Close()
	if got := getAsset(t, e.a, id); got.LocationID == nil || *got.LocationID != kantor {
		t.Fatalf("location changed after delete: %+v", got)
	}
	if len(listMoves(t, e.a, id)) != 0 {
		t.Fatal("movement still listed")
	}
	if h := hist(t, e.a, id); h.Items[0].Event != "movement_deleted" {
		t.Fatalf("history %+v", h.Items[0])
	}
	// Location with a deleted movement can be... still referenced? Deleting twice is 404.
	res = e.a.Do("DELETE", "/api/movements/"+m.ID, nil)
	if res.StatusCode != 404 {
		t.Fatalf("second delete %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestMovementSameLocation422(t *testing.T) {
	e := setup(t)
	gudang := createLocation(t, e.a, "Gudang")
	id := createAsset(t, e.a, gudang)
	res := e.a.Do("POST", "/api/assets/"+id+"/movements", map[string]any{"to_location_id": gudang, "date": "2026-05-01"})
	expect422(t, e.a, res, "to_location_id", "same_as_current")
	if len(listMoves(t, e.a, id)) != 0 {
		t.Fatal("movement stored")
	}
	res = e.a.Do("POST", "/api/assets/"+id+"/movements", map[string]any{"date": "2026-05-01"})
	expect422(t, e.a, res, "to_location_id", "required")
	other := createLocation(t, e.a, "Kantor")
	res = e.a.Do("POST", "/api/assets/"+id+"/movements", map[string]any{"to_location_id": other})
	expect422(t, e.a, res, "date", "required")
}

func TestMovementToOtherTenantLocation422(t *testing.T) {
	e := setup(t)
	foreign := createLocation(t, e.b, "Milik B")
	id := createAsset(t, e.a, "")
	res := e.a.Do("POST", "/api/assets/"+id+"/movements", map[string]any{"to_location_id": foreign, "date": "2026-05-01"})
	expect422(t, e.a, res, "to_location_id", "invalid")
	if got := getAsset(t, e.a, id); got.LocationID != nil {
		t.Fatalf("location set: %+v", got)
	}
	if len(listMoves(t, e.a, id)) != 0 {
		t.Fatal("movement stored")
	}
}

func TestCrossTenantMovementDelete404(t *testing.T) {
	e := setup(t)
	kantor := createLocation(t, e.a, "Kantor")
	id := createAsset(t, e.a, "")
	m := addMove(t, e.a, id, map[string]any{"to_location_id": kantor, "date": "2026-05-01"})
	bLoc := createLocation(t, e.b, "B")
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/assets/" + id + "/movements", nil},
		{"POST", "/api/assets/" + id + "/movements", map[string]any{"to_location_id": bLoc, "date": "2026-05-01"}},
		{"DELETE", "/api/movements/" + m.ID, nil},
	} {
		res := e.b.Do(tc.method, tc.path, tc.body)
		if res.StatusCode != 404 {
			t.Fatalf("%s %s: %d", tc.method, tc.path, res.StatusCode)
		}
		res.Body.Close()
	}
	if len(listMoves(t, e.a, id)) != 1 {
		t.Fatal("A's movement changed")
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM asset_history WHERE asset_id = $1 AND event = 'movement_deleted'`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("history rows %d %v", n, err)
	}
}
