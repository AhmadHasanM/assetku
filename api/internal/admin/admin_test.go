package admin_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/admin"
	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type errResp struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

type tenantItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	UserCount  int    `json:"user_count"`
	AssetCount int    `json:"asset_count"`
	CreatedAt  string `json:"created_at"`
}

type userItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Position string `json:"position"`
	Active   bool   `json:"active"`
}

const adminEmail, adminPass = "root@assetku.test", "admin-secret-1"

type env struct {
	pool  *pgxpool.Pool
	srv   *httptest.Server
	admin *testutil.APIClient
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	if err := admin.UpsertSuperAdmin(context.Background(), pool, adminEmail, "Root", adminPass); err != nil {
		t.Fatal(err)
	}
	as := auth.NewService(pool, false)
	ad := admin.NewService(pool, as)
	// Loopback is the trusted proxy so tests can pick a client IP via X-Real-IP.
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool,
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")},
		Register: func(m *http.ServeMux) {
			as.Register(m)
			ad.Register(m)
		}}))
	t.Cleanup(srv.Close)
	c := testutil.Client(t, srv.URL)
	res := c.Do("POST", "/api/admin/auth/login", map[string]string{"email": adminEmail, "password": adminPass})
	if res.StatusCode != 200 {
		t.Fatalf("admin login %d", res.StatusCode)
	}
	var body struct {
		Admin struct{ ID, Name, Email string } `json:"admin"`
	}
	c.JSON(res, &body)
	if body.Admin.Email != adminEmail || body.Admin.Name != "Root" || body.Admin.ID == "" {
		t.Fatalf("login body %+v", body)
	}
	return &env{pool: pool, srv: srv, admin: c}
}

func (e *env) createTenant(t *testing.T, name string) tenantItem {
	t.Helper()
	res := e.admin.Do("POST", "/api/admin/tenants", map[string]string{"name": name})
	if res.StatusCode != 201 {
		t.Fatalf("create tenant %d", res.StatusCode)
	}
	var ti tenantItem
	e.admin.JSON(res, &ti)
	return ti
}

func (e *env) createUser(t *testing.T, tenantID, email, pw string) (*http.Response, userItem) {
	t.Helper()
	res := e.admin.Do("POST", "/api/admin/tenants/"+tenantID+"/users",
		map[string]string{"name": "Budi", "email": email, "position": "Staff", "password": pw})
	var ui userItem
	if res.StatusCode == 201 {
		e.admin.JSON(res, &ui)
	}
	return res, ui
}

func TestCreateTenantAndUserThenUserCanLogin(t *testing.T) {
	e := setup(t)
	tn := e.createTenant(t, "Acme")
	if !tn.Active || tn.Name != "Acme" || tn.ID == "" {
		t.Fatalf("tenant %+v", tn)
	}
	res, u := e.createUser(t, tn.ID, "Budi@Acme.test", "password-1")
	if res.StatusCode != 201 || u.Email != "Budi@Acme.test" || !u.Active || u.Position != "Staff" {
		t.Fatalf("user %d %+v", res.StatusCode, u)
	}

	var list []tenantItem
	e.admin.JSON(e.admin.Do("GET", "/api/admin/tenants", nil), &list)
	if len(list) != 1 || list[0].UserCount != 1 || list[0].AssetCount != 0 || list[0].CreatedAt == "" {
		t.Fatalf("tenants %+v", list)
	}
	var users []userItem
	e.admin.JSON(e.admin.Do("GET", "/api/admin/tenants/"+tn.ID+"/users", nil), &users)
	if len(users) != 1 || users[0].ID != u.ID {
		t.Fatalf("users %+v", users)
	}

	uc := testutil.Client(t, e.srv.URL)
	if r := uc.Login("budi@acme.test", "password-1"); r.StatusCode != 200 {
		t.Fatalf("user login %d", r.StatusCode)
	}

	// PATCH tenant name and user fields.
	var patched tenantItem
	e.admin.JSON(e.admin.Do("PATCH", "/api/admin/tenants/"+tn.ID, map[string]any{"name": "Acme 2"}), &patched)
	if patched.Name != "Acme 2" || !patched.Active {
		t.Fatalf("patched %+v", patched)
	}
	var pu userItem
	e.admin.JSON(e.admin.Do("PATCH", "/api/admin/users/"+u.ID, map[string]any{"name": "Budi S", "position": "Manager"}), &pu)
	if pu.Name != "Budi S" || pu.Position != "Manager" || !pu.Active {
		t.Fatalf("patched user %+v", pu)
	}
	if r := e.admin.Do("PATCH", "/api/admin/users/00000000-0000-0000-0000-000000000000", map[string]any{"name": "x"}); r.StatusCode != 404 {
		t.Fatalf("unknown user %d", r.StatusCode)
	}
}

func TestDuplicateEmail422(t *testing.T) {
	e := setup(t)
	a, b := e.createTenant(t, "A"), e.createTenant(t, "B")
	if r, _ := e.createUser(t, a.ID, "dup@x.test", "password-1"); r.StatusCode != 201 {
		t.Fatalf("first %d", r.StatusCode)
	}
	r, _ := e.createUser(t, b.ID, "DUP@x.test", "password-1")
	var er errResp
	e.admin.JSON(r, &er)
	if r.StatusCode != 422 || er.Error.Fields["email"] != "taken" {
		t.Fatalf("got %d %+v", r.StatusCode, er)
	}
	r, _ = e.createUser(t, a.ID, "short@x.test", "short")
	e.admin.JSON(r, &er)
	if r.StatusCode != 422 || er.Error.Fields["password"] != "too_short" {
		t.Fatalf("pw %d %+v", r.StatusCode, er)
	}
}

func TestDeactivateTenantBlocksLogin(t *testing.T) {
	e := setup(t)
	tn := e.createTenant(t, "Acme")
	e.createUser(t, tn.ID, "u@acme.test", "password-1")
	uc := testutil.Client(t, e.srv.URL)
	if r := uc.Login("u@acme.test", "password-1"); r.StatusCode != 200 {
		t.Fatalf("login %d", r.StatusCode)
	}
	if r := e.admin.Do("PATCH", "/api/admin/tenants/"+tn.ID, map[string]any{"active": false}); r.StatusCode != 200 {
		t.Fatalf("patch %d", r.StatusCode)
	}
	if r := uc.Do("GET", "/api/auth/me", nil); r.StatusCode != 401 {
		t.Fatalf("existing session after deactivate %d", r.StatusCode)
	}
	if r := testutil.Client(t, e.srv.URL).Login("u@acme.test", "password-1"); r.StatusCode != 401 {
		t.Fatalf("new login after deactivate %d", r.StatusCode)
	}
}

func TestResetPasswordKillsSessions(t *testing.T) {
	e := setup(t)
	tn := e.createTenant(t, "Acme")
	_, u := e.createUser(t, tn.ID, "u@acme.test", "password-1")
	uc := testutil.Client(t, e.srv.URL)
	uc.Login("u@acme.test", "password-1")
	if r := uc.Do("GET", "/api/auth/me", nil); r.StatusCode != 200 {
		t.Fatalf("me %d", r.StatusCode)
	}
	if r := e.admin.Do("POST", "/api/admin/users/"+u.ID+"/reset-password", map[string]string{"password": "new-password-2"}); r.StatusCode != 204 {
		t.Fatalf("reset %d", r.StatusCode)
	}
	if r := uc.Do("GET", "/api/auth/me", nil); r.StatusCode != 401 {
		t.Fatalf("session after reset %d", r.StatusCode)
	}
	if r := testutil.Client(t, e.srv.URL).Login("u@acme.test", "password-1"); r.StatusCode != 401 {
		t.Fatalf("old password %d", r.StatusCode)
	}
	if r := testutil.Client(t, e.srv.URL).Login("u@acme.test", "new-password-2"); r.StatusCode != 200 {
		t.Fatalf("new password %d", r.StatusCode)
	}
	// Admin's own session is untouched.
	if r := e.admin.Do("GET", "/api/admin/auth/me", nil); r.StatusCode != 200 {
		t.Fatalf("admin me %d", r.StatusCode)
	}
	if r := e.admin.Do("POST", "/api/admin/users/"+u.ID+"/reset-password", map[string]string{"password": "x"}); r.StatusCode != 422 {
		t.Fatalf("weak %d", r.StatusCode)
	}
}

func TestUserSessionCannotAccessAdmin(t *testing.T) {
	e := setup(t)
	tn := e.createTenant(t, "Acme")
	e.createUser(t, tn.ID, "u@acme.test", "password-1")
	uc := testutil.Client(t, e.srv.URL)
	uc.Login("u@acme.test", "password-1")
	for _, p := range []string{"/api/admin/tenants", "/api/admin/auth/me", "/api/admin/tenants/" + tn.ID + "/users"} {
		if r := uc.Do("GET", p, nil); r.StatusCode != 401 {
			t.Fatalf("user on %s: %d", p, r.StatusCode)
		}
	}
	if r := uc.Do("POST", "/api/admin/tenants", map[string]string{"name": "X"}); r.StatusCode != 401 {
		t.Fatalf("user create tenant %d", r.StatusCode)
	}
	// Unauthenticated too.
	if r := testutil.Client(t, e.srv.URL).Do("GET", "/api/admin/tenants", nil); r.StatusCode != 401 {
		t.Fatalf("anon %d", r.StatusCode)
	}
	// Admin session cannot access tenant API.
	for _, p := range []string{"/api/auth/me"} {
		if r := e.admin.Do("GET", p, nil); r.StatusCode != 401 {
			t.Fatalf("admin on %s: %d", p, r.StatusCode)
		}
	}
}

func TestAdminLoginLogoutAndBadCredentials(t *testing.T) {
	e := setup(t)
	c := testutil.Client(t, e.srv.URL)
	r := c.Do("POST", "/api/admin/auth/login", map[string]string{"email": adminEmail, "password": "wrong"})
	var er errResp
	c.JSON(r, &er)
	if r.StatusCode != 401 || er.Error.Code != "invalid_credentials" {
		t.Fatalf("bad creds %d %+v", r.StatusCode, er)
	}
	if r := e.admin.Do("POST", "/api/admin/auth/logout", nil); r.StatusCode != 204 {
		t.Fatalf("logout %d", r.StatusCode)
	}
	if r := e.admin.Do("GET", "/api/admin/auth/me", nil); r.StatusCode != 401 {
		t.Fatalf("me after logout %d", r.StatusCode)
	}
	// A user-login endpoint must not accept admin credentials.
	if r := testutil.Client(t, e.srv.URL).Login(adminEmail, adminPass); r.StatusCode != 401 {
		t.Fatalf("admin creds on user login %d", r.StatusCode)
	}
}

func TestAdminLoginIPRateLimitAcrossEmails(t *testing.T) {
	e := setup(t)
	c := testutil.Client(t, e.srv.URL)
	c.Header.Set("X-Real-IP", "198.51.100.9")
	login := func(c *testutil.APIClient, email, pw string) *http.Response {
		return c.Do("POST", "/api/admin/auth/login", map[string]string{"email": email, "password": pw})
	}
	for i := 0; i < 50; i++ {
		r := login(c, fmt.Sprintf("spray%d@assetku.test", i), "wrong")
		r.Body.Close()
		if r.StatusCode != 401 {
			t.Fatalf("attempt %d status %d", i+1, r.StatusCode)
		}
	}
	r := login(c, adminEmail, adminPass)
	var er errResp
	c.JSON(r, &er)
	if r.StatusCode != 429 || er.Error.Code != "rate_limited" {
		t.Fatalf("51st: %d %+v", r.StatusCode, er)
	}
	other := testutil.Client(t, e.srv.URL)
	other.Header.Set("X-Real-IP", "198.51.100.10")
	if r := login(other, adminEmail, adminPass); r.StatusCode != 200 {
		t.Fatalf("other ip %d", r.StatusCode)
	} else {
		r.Body.Close()
	}
}

func TestAdminReloginDeletesPreviousSession(t *testing.T) {
	e := setup(t) // e.admin already holds one session
	r := e.admin.Do("POST", "/api/admin/auth/login", map[string]string{"email": adminEmail, "password": adminPass})
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("relogin %d", r.StatusCode)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE subject_type='super_admin'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("admin sessions %d", n)
	}
}

func TestCreateSuperadminCommandIdempotent(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	if err := admin.UpsertSuperAdmin(ctx, pool, "Root@x.test", "Root", "first-password"); err != nil {
		t.Fatal(err)
	}
	if err := admin.UpsertSuperAdmin(ctx, pool, "root@x.test", "Root", "second-password"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM super_admins`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows %d %v", n, err)
	}
	as := auth.NewService(pool, false)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: admin.NewService(pool, as).Register}))
	defer srv.Close()
	login := func(pw string) int {
		return testutil.Client(t, srv.URL).Do("POST", "/api/admin/auth/login", map[string]string{"email": "root@x.test", "password": pw}).StatusCode
	}
	if login("first-password") != 401 || login("second-password") != 200 {
		t.Fatal("password was not updated")
	}
	if err := admin.UpsertSuperAdmin(ctx, pool, "a@x.test", "A", "short"); err == nil {
		t.Fatal("expected weak password error")
	}
}

func TestTenantAssetCount(t *testing.T) {
	e := setup(t)
	acme := e.createTenant(t, "Acme")
	beta := e.createTenant(t, "Beta")
	for _, row := range []struct{ tenant, tag string }{{acme.ID, "A-1"}, {acme.ID, "A-2"}, {beta.ID, "B-1"}} {
		if _, err := e.pool.Exec(context.Background(),
			`INSERT INTO assets (tenant_id, name, asset_tag, category) VALUES ($1, 'x', $2, 'other')`, row.tenant, row.tag); err != nil {
			t.Fatal(err)
		}
	}
	var list []tenantItem
	e.admin.JSON(e.admin.Do("GET", "/api/admin/tenants", nil), &list)
	if len(list) != 2 || list[0].AssetCount != 2 || list[1].AssetCount != 1 {
		t.Fatalf("tenants %+v", list)
	}
	var one tenantItem
	e.admin.JSON(e.admin.Do("PATCH", "/api/admin/tenants/"+acme.ID, map[string]any{"name": "Acme 2"}), &one)
	if one.AssetCount != 2 {
		t.Fatalf("single tenant %+v", one)
	}
}
