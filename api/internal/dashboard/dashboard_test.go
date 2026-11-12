package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

var today = civil.Date{Year: 2026, Month: 11, Day: 12}

type env struct {
	pool    *pgxpool.Pool
	a, b    *testutil.APIClient
	tenantA uuid.UUID
	tenantB uuid.UUID
	url     string
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	svc := NewService(pool, as, asset.NewService(pool, as))
	svc.Today = func() civil.Date { return today }
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		svc.Register(m)
	}}))
	t.Cleanup(srv.Close)
	e := &env{pool: pool, url: srv.URL}
	e.tenantA = testutil.CreateTenant(t, pool, "A")
	e.tenantB = testutil.CreateTenant(t, pool, "B")
	testutil.CreateUser(t, pool, e.tenantA, "a@a.test", "password-1")
	testutil.CreateUser(t, pool, e.tenantB, "b@b.test", "password-1")
	e.a, e.b = testutil.Client(t, srv.URL), testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{e.a: "a@a.test", e.b: "b@b.test"} {
		res := c.Login(em, "password-1")
		if res.StatusCode != 200 {
			t.Fatalf("login %d", res.StatusCode)
		}
		res.Body.Close()
	}
	return e
}

func (e *env) asset(t *testing.T, tenant uuid.UUID, tag, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.pool.QueryRow(t.Context(), `INSERT INTO assets (tenant_id, name, asset_tag, category, status)
		VALUES ($1,$2,$2,'vehicle',$3) RETURNING id`, tenant, tag, status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) record(t *testing.T, tenant, asset uuid.UUID, date string, next *string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(), `INSERT INTO maintenance_records (tenant_id, asset_id, date, type, next_date)
		VALUES ($1,$2,$3,'preventive',$4)`, tenant, asset, date, next)
	if err != nil {
		t.Fatal(err)
	}
}

func sp(s string) *string { return &s }

func fetch(t *testing.T, c *testutil.APIClient) Dashboard {
	t.Helper()
	res := c.Do("GET", "/api/dashboard", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	var d Dashboard
	c.JSON(res, &d)
	return d
}

func TestDashboardShape(t *testing.T) {
	e := setup(t)
	avail := e.asset(t, e.tenantA, "AST-AVAIL", "available")
	e.asset(t, e.tenantA, "AST-DAMAGED", "damaged")
	e.asset(t, e.tenantA, "AST-LOST", "lost")
	e.asset(t, e.tenantA, "AST-ARCH", "archived")
	e.record(t, e.tenantA, avail, "2026-11-01", sp("2026-11-17"))

	d := fetch(t, e.a)
	if len(d.Attention) != 2 {
		t.Fatalf("attention %d", len(d.Attention))
	}
	if d.Attention[0].AssetTag != "AST-LOST" || d.Attention[1].AssetTag != "AST-DAMAGED" {
		t.Fatalf("attention order %+v", d.Attention)
	}
	if len(d.UpcomingMaintenance) != 1 || d.UpcomingMaintenance[0].Overdue ||
		d.UpcomingMaintenance[0].NextDate != (civil.Date{Year: 2026, Month: 11, Day: 17}) ||
		d.UpcomingMaintenance[0].AssetTag != "AST-AVAIL" {
		t.Fatalf("upcoming %+v", d.UpcomingMaintenance)
	}
	if len(d.ByCategory) != 6 || d.ByCategory[1].Key != "vehicle" || d.ByCategory[1].Count != 4 || d.ByCategory[0].Count != 0 {
		t.Fatalf("by_category %+v", d.ByCategory)
	}
	if len(d.ByStatus) != 6 || d.ByStatus[0].Key != "available" || d.ByStatus[5].Key != "archived" || d.ByStatus[5].Count != 1 {
		t.Fatalf("by_status %+v", d.ByStatus)
	}
	if d.Summary.Total != 4 || d.Summary.Available != 1 || d.Summary.NeedsAttention != 2 {
		t.Fatalf("summary %+v", d.Summary)
	}
}

func TestUpcomingOverdueWindowAndLatestRule(t *testing.T) {
	e := setup(t)
	over := e.asset(t, e.tenantA, "AST-OVER", "available")
	e.record(t, e.tenantA, over, "2026-10-01", sp("2026-11-11")) // yesterday: overdue
	edge := e.asset(t, e.tenantA, "AST-EDGE", "available")
	e.record(t, e.tenantA, edge, "2026-10-01", sp("2026-12-12")) // today+30: included
	far := e.asset(t, e.tenantA, "AST-FAR", "available")
	e.record(t, e.tenantA, far, "2026-10-01", sp("2026-12-13")) // today+31: excluded
	superseded := e.asset(t, e.tenantA, "AST-SUPER", "available")
	e.record(t, e.tenantA, superseded, "2026-09-01", sp("2026-11-15")) // older, in window
	e.record(t, e.tenantA, superseded, "2026-10-01", nil)              // newer, no next_date
	todayAsset := e.asset(t, e.tenantA, "AST-TODAY", "available")
	e.record(t, e.tenantA, todayAsset, "2026-10-01", sp("2026-11-12"))

	d := fetch(t, e.a)
	var tags []string
	for _, u := range d.UpcomingMaintenance {
		tags = append(tags, u.AssetTag)
		if u.Overdue != (u.AssetTag == "AST-OVER") {
			t.Fatalf("overdue wrong for %+v", u)
		}
	}
	want := []string{"AST-OVER", "AST-TODAY", "AST-EDGE"}
	if len(tags) != len(want) {
		t.Fatalf("tags %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("tags %v, want %v", tags, want)
		}
	}
}

func TestUpcomingExcludesArchivedAndLost(t *testing.T) {
	e := setup(t)
	arch := e.asset(t, e.tenantA, "AST-ARCH", "archived")
	e.record(t, e.tenantA, arch, "2026-10-01", sp("2026-11-01"))
	lost := e.asset(t, e.tenantA, "AST-LOST", "lost")
	e.record(t, e.tenantA, lost, "2026-10-01", sp("2026-11-20"))
	live := e.asset(t, e.tenantA, "AST-LIVE", "in_use")
	e.record(t, e.tenantA, live, "2026-10-01", sp("2026-11-20"))

	d := fetch(t, e.a)
	if len(d.UpcomingMaintenance) != 1 || d.UpcomingMaintenance[0].AssetTag != "AST-LIVE" {
		t.Fatalf("upcoming %+v", d.UpcomingMaintenance)
	}
}

func TestDashboardTenantIsolation(t *testing.T) {
	e := setup(t)
	b1 := e.asset(t, e.tenantB, "AST-BBBB", "damaged")
	e.record(t, e.tenantB, b1, "2026-11-01", sp("2026-11-13"))
	e.asset(t, e.tenantA, "AST-AAAA", "available")

	d := fetch(t, e.a)
	if len(d.Attention) != 0 || len(d.UpcomingMaintenance) != 0 || d.Summary.Total != 1 {
		t.Fatalf("leak: %+v", d)
	}
	for _, kc := range d.ByStatus {
		if kc.Key == "damaged" && kc.Count != 0 {
			t.Fatalf("by_status leak %+v", d.ByStatus)
		}
	}
	db := fetch(t, e.b)
	if len(db.Attention) != 1 || len(db.UpcomingMaintenance) != 1 {
		t.Fatalf("tenant B own data missing: %+v", db)
	}
}

func TestDashboardRequiresLogin(t *testing.T) {
	e := setup(t)
	res := testutil.Client(t, e.url).Do("GET", "/api/dashboard", nil)
	if res.StatusCode != 401 {
		t.Fatalf("status %d, want 401", res.StatusCode)
	}
}

func TestAttentionLocationName(t *testing.T) {
	e := setup(t)
	var loc uuid.UUID
	if err := e.pool.QueryRow(t.Context(), `INSERT INTO locations (tenant_id, name) VALUES ($1,'HQ') RETURNING id`, e.tenantA).Scan(&loc); err != nil {
		t.Fatal(err)
	}
	id := e.asset(t, e.tenantA, "AST-LOC", "maintenance")
	e.asset(t, e.tenantA, "AST-NOLOC", "damaged")
	if _, err := e.pool.Exec(t.Context(), `UPDATE assets SET location_id=$1 WHERE id=$2`, loc, id); err != nil {
		t.Fatal(err)
	}
	for _, it := range fetch(t, e.a).Attention {
		if (it.AssetTag == "AST-LOC") != (it.LocationName != nil && *it.LocationName == "HQ") {
			t.Fatalf("location_name wrong: %+v", it)
		}
	}
}
