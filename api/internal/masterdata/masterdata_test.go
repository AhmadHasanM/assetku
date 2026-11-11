package masterdata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
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

type env struct {
	pool    *pgxpool.Pool
	svc     *masterdata.Service
	srv     *httptest.Server
	tenantA uuid.UUID
	tenantB uuid.UUID
	a, b    *testutil.APIClient
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	md := masterdata.NewService(pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		md.Register(m)
	}}))
	t.Cleanup(srv.Close)
	e := &env{pool: pool, svc: md, srv: srv}
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

func TestCRUDLocations(t *testing.T) {
	e := setup(t)
	res := e.a.Do("POST", "/api/locations", map[string]string{"name": "  Gudang A ", "description": "Lantai 1", "bogus": "x"})
	if res.StatusCode != 201 {
		t.Fatalf("create %d", res.StatusCode)
	}
	var loc map[string]any
	e.a.JSON(res, &loc)
	id, _ := loc["id"].(string)
	if id == "" || loc["name"] != "Gudang A" || loc["description"] != "Lantai 1" || loc["asset_count"] != float64(0) {
		t.Fatalf("created %+v", loc)
	}
	res = e.a.Do("POST", "/api/locations", map[string]string{"name": "Aula"})
	res.Body.Close()

	var list []map[string]any
	res = e.a.Do("GET", "/api/locations", nil)
	e.a.JSON(res, &list)
	if len(list) != 2 || list[0]["name"] != "Aula" || list[1]["name"] != "Gudang A" {
		t.Fatalf("list %+v", list)
	}
	res = e.a.Do("GET", "/api/locations?q=gud", nil)
	e.a.JSON(res, &list)
	if len(list) != 1 || list[0]["id"] != id {
		t.Fatalf("search %+v", list)
	}
	res = e.a.Do("GET", "/api/locations?q=%25", nil)
	e.a.JSON(res, &list)
	if len(list) != 0 {
		t.Fatalf("wildcard must be literal: %+v", list)
	}

	res = e.a.Do("PATCH", "/api/locations/"+id, map[string]string{"description": "Lantai 2"})
	if res.StatusCode != 200 {
		t.Fatalf("patch %d", res.StatusCode)
	}
	e.a.JSON(res, &loc)
	if loc["name"] != "Gudang A" || loc["description"] != "Lantai 2" {
		t.Fatalf("patched %+v", loc)
	}
	res = e.a.Do("PATCH", "/api/locations/"+id, map[string]string{"name": "  "})
	var er errResp
	if res.StatusCode != 422 {
		t.Fatalf("empty patch name %d", res.StatusCode)
	}
	e.a.JSON(res, &er)
	if er.Error.Fields["name"] != "required" {
		t.Fatalf("%+v", er)
	}
	res = e.a.Do("POST", "/api/locations", map[string]string{"name": ""})
	e.a.JSON(res, &er)
	if res.StatusCode != 422 || er.Error.Fields["name"] != "required" {
		t.Fatalf("empty create %d %+v", res.StatusCode, er)
	}

	res = e.a.Do("DELETE", "/api/locations/"+id, nil)
	if res.StatusCode != 204 {
		t.Fatalf("delete %d", res.StatusCode)
	}
	res = e.a.Do("DELETE", "/api/locations/"+id, nil)
	if res.StatusCode != 404 {
		t.Fatalf("delete again %d", res.StatusCode)
	}
}

func TestSuppliersAndOrganizationsFields(t *testing.T) {
	e := setup(t)
	res := e.a.Do("POST", "/api/suppliers", map[string]string{"name": "PT Maju", "contact_name": "Siti", "phone": "0812", "email": "s@maju.test", "address": "Jl. Mawar"})
	var s map[string]any
	if res.StatusCode != 201 {
		t.Fatalf("create supplier %d", res.StatusCode)
	}
	e.a.JSON(res, &s)
	if s["contact_name"] != "Siti" || s["phone"] != "0812" || s["email"] != "s@maju.test" || s["address"] != "Jl. Mawar" {
		t.Fatalf("supplier %+v", s)
	}
	res = e.a.Do("POST", "/api/organizations", map[string]string{"name": "Finance", "description": "ignored"})
	var o map[string]any
	e.a.JSON(res, &o)
	if _, has := o["description"]; has || o["name"] != "Finance" {
		t.Fatalf("org %+v", o)
	}
}

func TestUnauthenticated401(t *testing.T) {
	e := setup(t)
	c := testutil.Client(t, e.srv.URL)
	res := c.Do("GET", "/api/locations", nil)
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func TestNameUniqueCaseInsensitivePerTenant(t *testing.T) {
	e := setup(t)
	for _, kind := range []string{"locations", "organizations", "suppliers"} {
		res := e.a.Do("POST", "/api/"+kind, map[string]string{"name": " Gudang "})
		res.Body.Close()
		if res.StatusCode != 201 {
			t.Fatalf("%s first %d", kind, res.StatusCode)
		}
		var er errResp
		res = e.a.Do("POST", "/api/"+kind, map[string]string{"name": "gudang"})
		e.a.JSON(res, &er)
		if res.StatusCode != 422 || er.Error.Fields["name"] != "taken" {
			t.Fatalf("%s dup %d %+v", kind, res.StatusCode, er)
		}
		res = e.b.Do("POST", "/api/"+kind, map[string]string{"name": "gudang"})
		res.Body.Close()
		if res.StatusCode != 201 {
			t.Fatalf("%s other tenant %d", kind, res.StatusCode)
		}
	}
	// PATCH rename onto an existing name is also taken.
	res := e.a.Do("POST", "/api/locations", map[string]string{"name": "Lain"})
	var loc map[string]any
	e.a.JSON(res, &loc)
	var er errResp
	res = e.a.Do("PATCH", "/api/locations/"+loc["id"].(string), map[string]string{"name": "GUDANG"})
	e.a.JSON(res, &er)
	if res.StatusCode != 422 || er.Error.Fields["name"] != "taken" {
		t.Fatalf("patch dup %d %+v", res.StatusCode, er)
	}
}

func TestCrossTenantPatchDelete404(t *testing.T) {
	e := setup(t)
	res := e.a.Do("POST", "/api/suppliers", map[string]string{"name": "Rahasia"})
	var s map[string]any
	e.a.JSON(res, &s)
	id := s["id"].(string)

	res = e.b.Do("PATCH", "/api/suppliers/"+id, map[string]string{"name": "Hacked"})
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("patch %d", res.StatusCode)
	}
	res = e.b.Do("DELETE", "/api/suppliers/"+id, nil)
	var er errResp
	e.b.JSON(res, &er)
	if res.StatusCode != 404 || er.Error.Code != "not_found" {
		t.Fatalf("delete %d %+v", res.StatusCode, er)
	}
	res = e.b.Do("PATCH", "/api/suppliers/not-a-uuid", map[string]string{"name": "x"})
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("bad id %d", res.StatusCode)
	}
	var list []map[string]any
	res = e.b.Do("GET", "/api/suppliers", nil)
	e.b.JSON(res, &list)
	if len(list) != 0 {
		t.Fatalf("B sees A data: %+v", list)
	}
	res = e.a.Do("GET", "/api/suppliers", nil)
	e.a.JSON(res, &list)
	if len(list) != 1 || list[0]["name"] != "Rahasia" {
		t.Fatalf("A data changed: %+v", list)
	}
}

func TestFindOrCreateByName(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	id1, created, err := e.svc.FindOrCreateByName(ctx, tx, e.tenantA, masterdata.Locations, "Kantor Pusat")
	if err != nil || !created || id1 == uuid.Nil {
		t.Fatalf("first: %v %v %v", id1, created, err)
	}
	id2, created, err := e.svc.FindOrCreateByName(ctx, tx, e.tenantA, masterdata.Locations, "  KANTOR pusat")
	if err != nil || created || id2 != id1 {
		t.Fatalf("second: %v %v %v", id2, created, err)
	}
	if _, _, err := e.svc.FindOrCreateByName(ctx, tx, e.tenantA, masterdata.Locations, "   "); err == nil {
		t.Fatal("blank name must error")
	}
	idB, created, err := e.svc.FindOrCreateByName(ctx, tx, e.tenantB, masterdata.Locations, "Kantor Pusat")
	if err != nil || !created || idB == id1 {
		t.Fatalf("other tenant: %v %v %v", idB, created, err)
	}

	got, ok, err := e.svc.FindByName(ctx, tx, e.tenantA, masterdata.Locations, " kantor PUSAT ")
	if err != nil || !ok || got != id1 {
		t.Fatalf("find: %v %v %v", got, ok, err)
	}
	if _, ok, err := e.svc.FindByName(ctx, tx, e.tenantA, masterdata.Suppliers, "Kantor Pusat"); err != nil || ok {
		t.Fatalf("find other kind: %v %v", ok, err)
	}
	// Also works directly on the pool.
	if got, ok, err := e.svc.FindByName(ctx, e.pool, e.tenantA, masterdata.Locations, "kantor pusat"); err != nil || ok {
		t.Fatalf("uncommitted row visible via pool: %v %v %v", got, ok, err)
	}
}
