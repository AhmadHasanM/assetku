package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"assetku/api/internal/httpx"
)

type fakeResolver struct {
	p       httpx.Principal
	refresh bool
	err     error
}

func (f fakeResolver) ResolveUser(ctx context.Context, token string) (httpx.Principal, bool, error) {
	if token != "good" {
		return httpx.Principal{}, false, httpx.ErrUnauthenticated
	}
	return f.p, f.refresh, f.err
}
func (f fakeResolver) ResolveAdmin(ctx context.Context, token string) (httpx.AdminPrincipal, bool, error) {
	return httpx.AdminPrincipal{}, false, httpx.ErrUnauthenticated
}
func (f fakeResolver) SessionCookie(token string) *http.Cookie {
	return &http.Cookie{Name: "session", Value: token, Path: "/"}
}

func TestRequireUser(t *testing.T) {
	want := httpx.Principal{UserID: uuid.New(), TenantID: uuid.New(), Name: "N"}
	h := httpx.RequireUser(fakeResolver{p: want, refresh: true})(httpx.H(func(w http.ResponseWriter, r *http.Request) error {
		if got := httpx.PrincipalFrom(r.Context()); got != want {
			t.Errorf("principal %+v", got)
		}
		w.WriteHeader(204)
		return nil
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), `"unauthenticated"`) {
		t.Fatalf("no cookie: %d %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "bad"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("bad token: %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "good"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("good token: %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Fatalf("expected refreshed session cookie, got %v", rec.Result().Cookies())
	}
}

func TestCSRF(t *testing.T) {
	r := httpx.NewRouter(httpx.Deps{Register: func(mux *http.ServeMux) {
		mux.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	}})

	// GET sets the cookie.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))
	var tok *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" {
			tok = c
		}
	}
	if tok == nil || tok.HttpOnly || tok.Value == "" {
		t.Fatalf("csrf cookie: %+v", tok)
	}

	post := func(cookie, header string) int {
		req := httptest.NewRequest("POST", "/x", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "csrf_token", Value: cookie})
		}
		if header != "" {
			req.Header.Set("X-CSRF-Token", header)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := post("", ""); c != 403 {
		t.Fatalf("no cookie/header: %d", c)
	}
	if c := post("abc", ""); c != 403 {
		t.Fatalf("no header: %d", c)
	}
	if c := post("abc", "xyz"); c != 403 {
		t.Fatalf("mismatch: %d", c)
	}
	if c := post("abc", "abc"); c != 204 {
		t.Fatalf("match: %d", c)
	}
}
