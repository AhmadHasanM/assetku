package maintenance_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/maintenance"
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

type record struct {
	ID            string  `json:"id"`
	Date          string  `json:"date"`
	Type          string  `json:"type"`
	PerformedBy   string  `json:"performed_by"`
	Cost          *string `json:"cost"`
	NextDate      *string `json:"next_date"`
	ReminderDays  *int    `json:"reminder_days"`
	Notes         string  `json:"notes"`
	CreatedByName string  `json:"created_by_name"`
}

type listResp struct {
	Items   []record `json:"items"`
	NextDue *struct {
		Date     string `json:"date"`
		RecordID string `json:"record_id"`
	} `json:"next_due"`
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
	mt := maintenance.NewService(pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		md.Register(m)
		assets.Register(m)
		hist.Register(m)
		mt.Register(m)
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

func createAsset(t *testing.T, c *testutil.APIClient) string {
	t.Helper()
	res := c.Do("POST", "/api/assets", map[string]any{"name": "Laptop", "category": "it_equipment", "status": "available"})
	if res.StatusCode != 201 {
		t.Fatalf("create asset %d", res.StatusCode)
	}
	var a struct {
		ID string `json:"id"`
	}
	c.JSON(res, &a)
	return a.ID
}

func addRecord(t *testing.T, c *testutil.APIClient, assetID string, body map[string]any) record {
	t.Helper()
	res := c.Do("POST", "/api/assets/"+assetID+"/maintenance", body)
	if res.StatusCode != 201 {
		var er errResp
		c.JSON(res, &er)
		t.Fatalf("add maintenance %d %+v", res.StatusCode, er)
	}
	var r record
	c.JSON(res, &r)
	return r
}

func list(t *testing.T, c *testutil.APIClient, assetID string) listResp {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+assetID+"/maintenance", nil)
	if res.StatusCode != 200 {
		t.Fatalf("list %d", res.StatusCode)
	}
	var l listResp
	c.JSON(res, &l)
	return l
}

func history_(t *testing.T, c *testutil.APIClient, assetID string) histPage {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+assetID+"/history", nil)
	var h histPage
	c.JSON(res, &h)
	return h
}

func TestAddMaintenanceDefaultsReminder7(t *testing.T) {
	e := setup(t)
	id := createAsset(t, e.a)
	r := addRecord(t, e.a, id, map[string]any{"date": "2026-03-01", "type": "preventive", "cost": "150000.5", "next_date": "2026-09-01"})
	if r.ReminderDays == nil || *r.ReminderDays != 7 {
		t.Fatalf("reminder_days = %v, want 7", r.ReminderDays)
	}
	if r.Cost == nil || *r.Cost != "150000.50" {
		t.Fatalf("cost = %v, want string 150000.50", r.Cost)
	}
	if r.CreatedByName != "User a@a.test" {
		t.Fatalf("created_by_name %q", r.CreatedByName)
	}
	// Explicit value is kept; no next_date means no default.
	r2 := addRecord(t, e.a, id, map[string]any{"date": "2026-03-02", "type": "inspection", "next_date": "2026-04-01", "reminder_days": 0})
	if r2.ReminderDays == nil || *r2.ReminderDays != 0 {
		t.Fatalf("explicit reminder_days = %v, want 0", r2.ReminderDays)
	}
	r3 := addRecord(t, e.a, id, map[string]any{"date": "2026-03-03", "type": "other"})
	if r3.ReminderDays != nil || r3.NextDate != nil || r3.Cost != nil {
		t.Fatalf("expected nulls, got %+v", r3)
	}
	// Invalid type and negative cost.
	res := e.a.Do("POST", "/api/assets/"+id+"/maintenance", map[string]any{"date": "2026-03-03", "type": "bogus", "cost": "-1"})
	var er errResp
	if res.StatusCode != 422 {
		t.Fatalf("status %d", res.StatusCode)
	}
	e.a.JSON(res, &er)
	if er.Error.Fields["type"] != "invalid" || er.Error.Fields["cost"] != "invalid" {
		t.Fatalf("fields %+v", er.Error.Fields)
	}
	res = e.a.Do("POST", "/api/assets/"+id+"/maintenance", map[string]any{"type": "other"})
	e.a.JSON(res, &er)
	if res.StatusCode != 422 || er.Error.Fields["date"] != "required" {
		t.Fatalf("missing date: %d %+v", res.StatusCode, er.Error.Fields)
	}
}

func TestNextDateBeforeDate422(t *testing.T) {
	e := setup(t)
	id := createAsset(t, e.a)
	res := e.a.Do("POST", "/api/assets/"+id+"/maintenance", map[string]any{"date": "2026-03-10", "type": "preventive", "next_date": "2026-03-09"})
	var er errResp
	if res.StatusCode != 422 {
		t.Fatalf("status %d", res.StatusCode)
	}
	e.a.JSON(res, &er)
	if er.Error.Code != "validation" || er.Error.Fields["next_date"] != "before_date" {
		t.Fatalf("got %+v", er.Error)
	}
	if l := list(t, e.a, id); len(l.Items) != 0 {
		t.Fatal("record was stored")
	}
	// Same day is allowed.
	addRecord(t, e.a, id, map[string]any{"date": "2026-03-10", "type": "preventive", "next_date": "2026-03-10"})
}

func TestListOrderAndNextDueFromLatestRecord(t *testing.T) {
	e := setup(t)
	id := createAsset(t, e.a)
	if l := list(t, e.a, id); l.NextDue != nil || l.Items == nil || len(l.Items) != 0 {
		t.Fatalf("empty list: %+v", l)
	}
	old := addRecord(t, e.a, id, map[string]any{"date": "2026-01-01", "type": "preventive", "next_date": "2026-07-01"})
	if l := list(t, e.a, id); l.NextDue == nil || l.NextDue.RecordID != old.ID || l.NextDue.Date != "2026-07-01" {
		t.Fatalf("next_due %+v", l.NextDue)
	}
	// A newer record without next_date supersedes the older one: nothing due.
	newer := addRecord(t, e.a, id, map[string]any{"date": "2026-02-01", "type": "corrective"})
	l := list(t, e.a, id)
	if l.NextDue != nil {
		t.Fatalf("next_due should be nil, got %+v", l.NextDue)
	}
	if len(l.Items) != 2 || l.Items[0].ID != newer.ID || l.Items[1].ID != old.ID {
		t.Fatalf("order %+v", l.Items)
	}
	// Same date: created_at DESC breaks the tie.
	tie := addRecord(t, e.a, id, map[string]any{"date": "2026-02-01", "type": "inspection", "next_date": "2026-08-01"})
	l = list(t, e.a, id)
	if l.Items[0].ID != tie.ID || l.NextDue == nil || l.NextDue.RecordID != tie.ID {
		t.Fatalf("tie-break: %+v", l)
	}
}

func TestDeleteMaintenanceHistory(t *testing.T) {
	e := setup(t)
	id := createAsset(t, e.a)
	r := addRecord(t, e.a, id, map[string]any{"date": "2026-03-01", "type": "corrective"})
	res := e.a.Do("DELETE", "/api/maintenance/"+r.ID, nil)
	if res.StatusCode != 204 {
		t.Fatalf("delete %d", res.StatusCode)
	}
	res.Body.Close()
	if l := list(t, e.a, id); len(l.Items) != 0 {
		t.Fatal("record still listed")
	}
	h := history_(t, e.a, id)
	if len(h.Items) < 3 {
		t.Fatalf("history %+v", h.Items)
	}
	if h.Items[0].Event != "maintenance_deleted" || h.Items[0].Detail["type"] != "corrective" || h.Items[0].Detail["date"] != "2026-03-01" {
		t.Fatalf("deleted entry %+v", h.Items[0])
	}
	if h.Items[1].Event != "maintenance_added" || h.Items[1].Detail["record_id"] != r.ID ||
		h.Items[1].Detail["type"] != "corrective" || h.Items[1].Detail["date"] != "2026-03-01" {
		t.Fatalf("added entry %+v", h.Items[1])
	}
	res = e.a.Do("DELETE", "/api/maintenance/"+r.ID, nil)
	if res.StatusCode != 404 {
		t.Fatalf("second delete %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestCrossTenantMaintenanceDelete404(t *testing.T) {
	e := setup(t)
	id := createAsset(t, e.a)
	r := addRecord(t, e.a, id, map[string]any{"date": "2026-03-01", "type": "corrective"})
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/assets/" + id + "/maintenance"},
		{"DELETE", "/api/maintenance/" + r.ID},
	} {
		res := e.b.Do(tc.method, tc.path, nil)
		if res.StatusCode != 404 {
			t.Fatalf("%s %s: %d", tc.method, tc.path, res.StatusCode)
		}
		res.Body.Close()
	}
	res := e.b.Do("POST", "/api/assets/"+id+"/maintenance", map[string]any{"date": "2026-03-01", "type": "other"})
	if res.StatusCode != 404 {
		t.Fatalf("cross-tenant post %d", res.StatusCode)
	}
	res.Body.Close()
	if l := list(t, e.a, id); len(l.Items) != 1 {
		t.Fatalf("record of A changed: %+v", l.Items)
	}
	res = e.a.Do("DELETE", "/api/maintenance/not-a-uuid", nil)
	if res.StatusCode != 404 {
		t.Fatalf("bad id %d", res.StatusCode)
	}
	res.Body.Close()
}
