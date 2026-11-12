package admin_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"assetku/api/internal/admin"
	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestSeedDemoIdempotent(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	in := admin.DemoSeed{Tenant: "PT Demo", Email: "demo@assetku.test", Password: "demo12345"}
	for i := 0; i < 2; i++ {
		if err := admin.SeedDemo(ctx, pool, in); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM tenants`); n != 1 {
		t.Fatalf("tenants %d", n)
	}
	if n := count(`SELECT count(*) FROM users`); n != 1 {
		t.Fatalf("users %d", n)
	}
	if n := count(`SELECT count(*) FROM locations l JOIN users u ON u.tenant_id = l.tenant_id`); n != 3 {
		t.Fatalf("locations %d", n)
	}
	if n := count(`SELECT count(*) FROM locations WHERE name = 'Gudang Bekasi'`); n != 1 {
		t.Fatalf("Gudang Bekasi %d", n)
	}
	if n := count(`SELECT count(*) FROM organizations`); n != 1 {
		t.Fatalf("organizations %d", n)
	}

	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: auth.NewService(pool, false).Register}))
	defer srv.Close()
	login := func(pw string) int {
		return testutil.Client(t, srv.URL).Do("POST", "/api/auth/login", map[string]string{"email": in.Email, "password": pw}).StatusCode
	}
	if c := login("demo12345"); c != 200 {
		t.Fatalf("login %d", c)
	}

	// A re-run with a new password resets it and the preferences (handy before a demo).
	if _, err := pool.Exec(ctx, `UPDATE users SET locale = 'en', theme = 'light'`); err != nil {
		t.Fatal(err)
	}
	in.Password = "another-pass"
	if err := admin.SeedDemo(ctx, pool, in); err != nil {
		t.Fatal(err)
	}
	if login("demo12345") != 401 || login("another-pass") != 200 {
		t.Fatal("password was not reset")
	}
	var locale, theme string
	if err := pool.QueryRow(ctx, `SELECT locale, theme FROM users`).Scan(&locale, &theme); err != nil || locale != "id" || theme != "dark" {
		t.Fatalf("preferences %s/%s %v", locale, theme, err)
	}

	// The email is taken by a user of another tenant: refuse instead of moving it.
	if err := admin.SeedDemo(ctx, pool, admin.DemoSeed{Tenant: "PT Lain", Email: "DEMO@assetku.test", Password: "demo12345"}); err == nil {
		t.Fatal("expected error for email of another tenant")
	}
	if err := admin.SeedDemo(ctx, pool, admin.DemoSeed{Tenant: "PT Demo", Email: "x@assetku.test", Password: "short"}); err == nil {
		t.Fatal("expected weak password error")
	}
}
