package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

var ctx = context.Background()

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func newAsset(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID, name, tag string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `INSERT INTO assets (tenant_id, name, asset_tag, category, status)
		VALUES ($1,$2,$3,'it_equipment','available') RETURNING id`, tenant, name, tag).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func addRec(t *testing.T, pool *pgxpool.Pool, tenant, asset uuid.UUID, date string, next *string, days *int) {
	t.Helper()
	mustExec(t, pool, `INSERT INTO maintenance_records (tenant_id, asset_id, date, type, next_date, reminder_days)
		VALUES ($1,$2,$3,'preventive',$4,$5)`, tenant, asset, date, next, days)
}

func count(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

type fixture struct {
	pool          *pgxpool.Pool
	tenant, asset uuid.UUID
	u1, u2        uuid.UUID
}

func setup(t *testing.T) *fixture {
	pool := testutil.NewDB(t)
	f := &fixture{pool: pool}
	f.tenant = testutil.CreateTenant(t, pool, "A")
	f.u1 = testutil.CreateUser(t, pool, f.tenant, "u1@a.test", "password-1")
	f.u2 = testutil.CreateUser(t, pool, f.tenant, "u2@a.test", "password-1")
	inactive := testutil.CreateUser(t, pool, f.tenant, "u3@a.test", "password-1")
	mustExec(t, pool, `UPDATE users SET active=false WHERE id=$1`, inactive)
	f.asset = newAsset(t, pool, f.tenant, "Laptop", "AST-AAAAAAAA")
	return f
}

func TestReminderFiresOnDayWindowStart(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	n, err := RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 11})
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := count(t, f.pool); got != 2 {
		t.Fatalf("notifications %d", got)
	}
	var bid, ben string
	f.pool.QueryRow(ctx, `SELECT body_id, body_en FROM notifications LIMIT 1`).Scan(&bid, &ben)
	if bid != "Maintenance Laptop (AST-AAAAAAAA) jatuh tempo 18 Nov 2026" || ben != "Maintenance for Laptop (AST-AAAAAAAA) is due 18 Nov 2026" {
		t.Fatalf("bodies %q %q", bid, ben)
	}
}

func TestReminderIndonesianMonth(t *testing.T) {
	if got := formatDate(civil.Date{Year: 2026, Month: 8, Day: 5}, monthsID); got != "05 Agu 2026" {
		t.Fatal(got)
	}
}

func TestReminderNotBeforeWindow(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	n, err := RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 10})
	if err != nil || n != 0 || count(t, f.pool) != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestReminderOnlyOnce(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	today := civil.Date{Year: 2026, Month: 11, Day: 11}
	RunReminders(ctx, f.pool, today)
	n, err := RunReminders(ctx, f.pool, today)
	if err != nil || n != 0 || count(t, f.pool) != 2 {
		t.Fatalf("n=%d err=%v total=%d", n, err, count(t, f.pool))
	}
}

func TestReminderOnlyLatestRecordPerAsset(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	addRec(t, f.pool, f.tenant, f.asset, "2026-06-01", nil, nil)
	n, err := RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 15})
	if err != nil || n != 0 || count(t, f.pool) != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestReminderOverdueStillFiresOnce(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	n, err := RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 12, Day: 30})
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestReminderSkipsInactiveTenant(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	mustExec(t, f.pool, `UPDATE tenants SET active=false WHERE id=$1`, f.tenant)
	n, err := RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 15})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestReminderConcurrentRunsNoDuplicates(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	today := civil.Date{Year: 2026, Month: 11, Day: 11}
	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go func() { RunReminders(ctx, f.pool, today); done <- struct{}{} }()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	if got := count(t, f.pool); got != 2 {
		t.Fatalf("notifications %d", got)
	}
}

func apiSetup(t *testing.T, f *fixture) (*testutil.APIClient, *testutil.APIClient) {
	as := auth.NewService(f.pool, false)
	svc := NewService(f.pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: f.pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		svc.Register(m)
	}}))
	t.Cleanup(srv.Close)
	c1, c2 := testutil.Client(t, srv.URL), testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{c1: "u1@a.test", c2: "u2@a.test"} {
		res := c.Login(em, "password-1")
		if res.StatusCode != 200 {
			t.Fatalf("login %d", res.StatusCode)
		}
		res.Body.Close()
	}
	return c1, c2
}

type listResp struct {
	Unread int `json:"unread"`
	Items  []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"items"`
}

func TestNotificationLocale(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 11})
	mustExec(t, f.pool, `UPDATE users SET locale='en' WHERE id=$1`, f.u2)
	c1, c2 := apiSetup(t, f)
	var l1, l2 listResp
	res := c1.Do("GET", "/api/notifications", nil)
	c1.JSON(res, &l1)
	res = c2.Do("GET", "/api/notifications", nil)
	c2.JSON(res, &l2)
	if l1.Unread != 1 || len(l1.Items) != 1 || l1.Items[0].Title != "Pengingat maintenance" ||
		l1.Items[0].Body != "Maintenance Laptop (AST-AAAAAAAA) jatuh tempo 18 Nov 2026" {
		t.Fatalf("id: %+v", l1)
	}
	if l2.Items[0].Title != "Maintenance reminder" || l2.Items[0].Body != "Maintenance for Laptop (AST-AAAAAAAA) is due 18 Nov 2026" {
		t.Fatalf("en: %+v", l2)
	}
}

func TestMarkReadOtherUser404(t *testing.T) {
	f := setup(t)
	addRec(t, f.pool, f.tenant, f.asset, "2026-05-01", sp("2026-11-18"), ip(7))
	RunReminders(ctx, f.pool, civil.Date{Year: 2026, Month: 11, Day: 11})
	c1, c2 := apiSetup(t, f)
	var l1 listResp
	res := c1.Do("GET", "/api/notifications", nil)
	c1.JSON(res, &l1)
	path := "/api/notifications/" + itoa(l1.Items[0].ID) + "/read"
	if res := c2.Do("POST", path, nil); res.StatusCode != 404 {
		t.Fatalf("other user %d", res.StatusCode)
	}
	if res := c1.Do("POST", path, nil); res.StatusCode != 204 {
		t.Fatalf("own %d", res.StatusCode)
	}
	res = c1.Do("GET", "/api/notifications", nil)
	c1.JSON(res, &l1)
	if l1.Unread != 0 {
		t.Fatalf("unread %d", l1.Unread)
	}
	if res := c2.Do("POST", "/api/notifications/read-all", nil); res.StatusCode != 204 {
		t.Fatalf("read-all %d", res.StatusCode)
	}
	var l2 listResp
	res = c2.Do("GET", "/api/notifications", nil)
	c2.JSON(res, &l2)
	if l2.Unread != 0 {
		t.Fatalf("unread %d", l2.Unread)
	}
}

func TestUnreadCountsBeyondTwenty(t *testing.T) {
	f := setup(t)
	mustExec(t, f.pool, `INSERT INTO notifications (tenant_id,user_id,type,title_id,title_en,body_id,body_en)
		SELECT $1,$2,'x','a','a','b','b' FROM generate_series(1,25)`, f.tenant, f.u1)
	c1, _ := apiSetup(t, f)
	var l listResp
	res := c1.Do("GET", "/api/notifications", nil)
	c1.JSON(res, &l)
	if l.Unread != 25 || len(l.Items) != 20 {
		t.Fatalf("unread %d items %d", l.Unread, len(l.Items))
	}
}

func TestSchedulerNextRun(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	got := nextRun(time.Date(2026, 11, 11, 6, 59, 0, 0, loc), loc)
	if want := time.Date(2026, 11, 11, 7, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("06:59 -> %v", got)
	}
	got = nextRun(time.Date(2026, 11, 11, 7, 0, 0, 0, loc), loc)
	if want := time.Date(2026, 11, 12, 7, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("07:00 -> %v", got)
	}
	// Input in another zone is interpreted in loc.
	got = nextRun(time.Date(2026, 11, 10, 23, 30, 0, 0, time.UTC), loc) // 06:30 WIB on the 11th
	if want := time.Date(2026, 11, 11, 7, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("utc -> %v", got)
	}
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }
