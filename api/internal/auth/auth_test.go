package auth_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type env struct {
	srv *httptest.Server
	c   *testutil.APIClient
}

type errResp struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

type meResp struct {
	User struct {
		ID, Name, Email, Position, Locale, Theme string
		Tenant                                   struct{ ID, Name string }
	}
}

func setup(t *testing.T) (*env, func(name, email, pw string)) {
	t.Helper()
	pool := testutil.NewDB(t)
	svc := auth.NewService(pool, false)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: svc.Register}))
	t.Cleanup(srv.Close)
	seed := func(tenant, email, pw string) {
		tid := testutil.CreateTenant(t, pool, tenant)
		testutil.CreateUser(t, pool, tid, email, pw)
	}
	return &env{srv: srv, c: testutil.Client(t, srv.URL)}, seed
}

func me(t *testing.T, e *env) meResp {
	t.Helper()
	res := e.c.Do("GET", "/api/auth/me", nil)
	if res.StatusCode != 200 {
		t.Fatalf("me status %d", res.StatusCode)
	}
	var m struct {
		User struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Position string `json:"position"`
			Locale   string `json:"locale"`
			Theme    string `json:"theme"`
			Tenant   struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"tenant"`
		} `json:"user"`
	}
	e.c.JSON(res, &m)
	var out meResp
	out.User.ID, out.User.Name, out.User.Email = m.User.ID, m.User.Name, m.User.Email
	out.User.Position, out.User.Locale, out.User.Theme = m.User.Position, m.User.Locale, m.User.Theme
	out.User.Tenant.ID, out.User.Tenant.Name = m.User.Tenant.ID, m.User.Tenant.Name
	return out
}

func TestLoginSuccessSetsSessionAndMe(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "Budi@Acme.test", "secret-pass")
	res := e.c.Login("budi@acme.test", "secret-pass")
	if res.StatusCode != 200 {
		t.Fatalf("login status %d", res.StatusCode)
	}
	var lr meResp
	e.c.JSON(res, &lr)
	if lr.User.Email != "Budi@Acme.test" {
		t.Fatalf("login body email %q", lr.User.Email)
	}
	var sess *struct{ httpOnly, lax bool }
	for _, ck := range res.Cookies() {
		if ck.Name == "session" {
			sess = &struct{ httpOnly, lax bool }{ck.HttpOnly, ck.SameSite == http.SameSiteLaxMode}
			if ck.Path != "/" || ck.MaxAge <= 0 {
				t.Fatalf("bad session cookie %+v", ck)
			}
		}
	}
	if sess == nil || !sess.httpOnly || !sess.lax {
		t.Fatalf("session cookie missing or not httpOnly/Lax: %+v", sess)
	}
	m := me(t, e)
	if m.User.Email != "Budi@Acme.test" || m.User.Tenant.Name != "Acme" || m.User.Locale != "id" || m.User.Theme != "dark" {
		t.Fatalf("me = %+v", m)
	}
}

func TestLoginWrongPassword401(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	res := e.c.Login("a@acme.test", "nope")
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 401 || er.Error.Code != "invalid_credentials" {
		t.Fatalf("got %d %+v", status, er)
	}
	res = e.c.Login("ghost@acme.test", "nope")
	e.c.JSON(res, &er)
	if res.StatusCode != 401 || er.Error.Code != "invalid_credentials" {
		t.Fatalf("unknown email: got %d %+v", res.StatusCode, er)
	}
}

func TestLoginInactiveTenant401(t *testing.T) {
	pool := testutil.NewDB(t)
	svc := auth.NewService(pool, false)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: svc.Register}))
	defer srv.Close()
	tid := testutil.CreateTenant(t, pool, "Acme")
	uid := testutil.CreateUser(t, pool, tid, "a@acme.test", "secret-pass")
	c := testutil.Client(t, srv.URL)
	if res := c.Login("a@acme.test", "secret-pass"); res.StatusCode != 200 {
		t.Fatalf("pre-check login %d", res.StatusCode)
	}

	// Deactivating the tenant kills the existing session and blocks new logins.
	if _, err := pool.Exec(t.Context(), `UPDATE tenants SET active=false WHERE id=$1`, tid); err != nil {
		t.Fatal(err)
	}
	if res := c.Do("GET", "/api/auth/me", nil); res.StatusCode != 401 {
		t.Fatalf("existing session after tenant deactivation: %d", res.StatusCode)
	}
	c2 := testutil.Client(t, srv.URL)
	res := c2.Login("a@acme.test", "secret-pass")
	var er errResp
	status := res.StatusCode
	c2.JSON(res, &er)
	if status != 401 || er.Error.Code != "invalid_credentials" {
		t.Fatalf("got %d %+v", status, er)
	}

	// Inactive user behaves the same.
	if _, err := pool.Exec(t.Context(), `UPDATE tenants SET active=true WHERE id=$1`, tid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE users SET active=false WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	res = c2.Login("a@acme.test", "secret-pass")
	status = res.StatusCode
	c2.JSON(res, &er)
	if status != 401 || er.Error.Code != "invalid_credentials" {
		t.Fatalf("inactive user: got %d %+v", status, er)
	}
}

func TestRateLimitAfter10Failures(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	for i := 0; i < 10; i++ {
		res := e.c.Login("a@acme.test", "wrong")
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("attempt %d status %d", i+1, res.StatusCode)
		}
	}
	res := e.c.Login("a@acme.test", "secret-pass")
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 429 || er.Error.Code != "rate_limited" {
		t.Fatalf("got %d %+v", status, er)
	}
	// A different email from the same IP is unaffected.
	seed("Other", "b@other.test", "secret-pass")
	res = e.c.Login("b@other.test", "secret-pass")
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("other email status %d", res.StatusCode)
	}
}

func TestPostWithoutCSRF403(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	res := e.c.Login("a@acme.test", "secret-pass")
	res.Body.Close()
	res = e.c.DoWithoutCSRF("PATCH", "/api/me", map[string]string{"name": "X"})
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 403 || er.Error.Code != "csrf" {
		t.Fatalf("got %d %+v", status, er)
	}
	if m := me(t, e); m.User.Name == "X" {
		t.Fatal("profile changed despite CSRF failure")
	}
}

func TestUpdateProfileLocaleTheme(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	e.c.Login("a@acme.test", "secret-pass").Body.Close()

	res := e.c.Do("PATCH", "/api/me", map[string]string{"locale": "en", "theme": "light", "position": "Manager"})
	if res.StatusCode != 200 {
		t.Fatalf("patch status %d", res.StatusCode)
	}
	res.Body.Close()
	m := me(t, e)
	if m.User.Locale != "en" || m.User.Theme != "light" || m.User.Position != "Manager" {
		t.Fatalf("me = %+v", m)
	}

	res = e.c.Do("PATCH", "/api/me", map[string]string{"locale": "fr"})
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 422 || er.Error.Fields["locale"] != "invalid" {
		t.Fatalf("got %d %+v", status, er)
	}
	if m := me(t, e); m.User.Locale != "en" {
		t.Fatalf("locale changed to %q", m.User.Locale)
	}
}

func TestChangePassword(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	e.c.Login("a@acme.test", "secret-pass").Body.Close()

	res := e.c.Do("POST", "/api/me/password", map[string]string{"current_password": "wrong", "new_password": "brand-new-pass"})
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 422 || er.Error.Fields["current_password"] != "invalid" {
		t.Fatalf("got %d %+v", status, er)
	}

	res = e.c.Do("POST", "/api/me/password", map[string]string{"current_password": "secret-pass", "new_password": "short"})
	status = res.StatusCode
	e.c.JSON(res, &er)
	if status != 422 || er.Error.Fields["new_password"] != "too_short" {
		t.Fatalf("short: got %d %+v", status, er)
	}

	res = e.c.Do("POST", "/api/me/password", map[string]string{"current_password": "secret-pass", "new_password": "brand-new-pass"})
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("change status %d", res.StatusCode)
	}

	c2 := testutil.Client(t, e.srv.URL)
	r := c2.Login("a@acme.test", "secret-pass")
	r.Body.Close()
	if r.StatusCode != 401 {
		t.Fatalf("old password login status %d", r.StatusCode)
	}
	r = c2.Login("a@acme.test", "brand-new-pass")
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("new password login status %d", r.StatusCode)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	e.c.Login("a@acme.test", "secret-pass").Body.Close()
	saved := e.c.Cookie("session")
	if saved == "" {
		t.Fatal("no session cookie")
	}
	res := e.c.Do("POST", "/api/auth/logout", nil)
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("logout status %d", res.StatusCode)
	}
	// Replay the saved token: the server-side session must be gone.
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: saved})
	replay, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	replay.Body.Close()
	if replay.StatusCode != 401 {
		t.Fatalf("replayed token status %d", replay.StatusCode)
	}
	res = e.c.Do("GET", "/api/auth/me", nil)
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 401 || er.Error.Code != "unauthenticated" {
		t.Fatalf("got %d %+v", status, er)
	}
}

func TestSlidingExtension(t *testing.T) {
	pool := testutil.NewDB(t)
	svc := auth.NewService(pool, false)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: svc.Register}))
	defer srv.Close()
	tid := testutil.CreateTenant(t, pool, "Acme")
	testutil.CreateUser(t, pool, tid, "a@acme.test", "secret-pass")
	c := testutil.Client(t, srv.URL)
	c.Login("a@acme.test", "secret-pass").Body.Close()

	// Sessions with > 15 days left are untouched; with < 15 days left they are renewed to ~30 days.
	var before, after string
	q := `SELECT to_char(expires_at, 'YYYY-MM-DD"T"HH24:MI:SS.US') FROM sessions`
	if err := pool.QueryRow(t.Context(), q).Scan(&before); err != nil {
		t.Fatal(err)
	}
	c.Do("GET", "/api/auth/me", nil).Body.Close()
	if err := pool.QueryRow(t.Context(), q).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("session extended too early: %s -> %s", before, after)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE sessions SET expires_at = now() + interval '10 days'`); err != nil {
		t.Fatal(err)
	}
	c.Do("GET", "/api/auth/me", nil).Body.Close()
	var days float64
	if err := pool.QueryRow(t.Context(), `SELECT extract(epoch FROM expires_at - now())/86400 FROM sessions`).Scan(&days); err != nil {
		t.Fatal(err)
	}
	if days < 29 || days > 30.1 {
		t.Fatalf("expected ~30 days left, got %.2f", days)
	}
}

func TestPasswordHelpers(t *testing.T) {
	h, err := auth.HashPassword("secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(h, "secret-pass") || auth.CheckPassword(h, "other") || auth.CheckPassword("garbage", "x") {
		t.Fatal("password check mismatch")
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	a := e.c
	b := testutil.Client(t, e.srv.URL)
	a.Login("a@acme.test", "secret-pass").Body.Close()
	b.Login("a@acme.test", "secret-pass").Body.Close()
	if r := b.Do("GET", "/api/auth/me", nil); r.StatusCode != 200 {
		t.Fatalf("B before: %d", r.StatusCode)
	}

	res := a.Do("POST", "/api/me/password", map[string]string{"current_password": "secret-pass", "new_password": "brand-new-pass"})
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("change status %d", res.StatusCode)
	}
	rb := b.Do("GET", "/api/auth/me", nil)
	rb.Body.Close()
	if rb.StatusCode != 401 {
		t.Fatalf("B after change: %d", rb.StatusCode)
	}
	ra := a.Do("GET", "/api/auth/me", nil)
	ra.Body.Close()
	if ra.StatusCode != 200 {
		t.Fatalf("A after change: %d", ra.StatusCode)
	}
}

func TestChangePasswordTooLong(t *testing.T) {
	e, seed := setup(t)
	seed("Acme", "a@acme.test", "secret-pass")
	e.c.Login("a@acme.test", "secret-pass").Body.Close()
	res := e.c.Do("POST", "/api/me/password", map[string]string{"current_password": "secret-pass", "new_password": strings.Repeat("x", 73)})
	var er errResp
	status := res.StatusCode
	e.c.JSON(res, &er)
	if status != 422 || er.Error.Fields["new_password"] != "too_long" {
		t.Fatalf("got %d %+v", status, er)
	}
	res = e.c.Do("POST", "/api/me/password", map[string]string{"current_password": "secret-pass", "new_password": strings.Repeat("x", 72)})
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("72 bytes status %d", res.StatusCode)
	}
}
