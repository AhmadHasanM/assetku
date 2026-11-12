package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// CreateTenant inserts an active tenant and returns its id.
func CreateTenant(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return id
}

// CreateUser inserts an active user. The password is hashed with the minimum
// bcrypt cost to keep tests fast; verification works with any cost.
func CreateUser(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, email, password string) uuid.UUID {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (tenant_id, name, email, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`,
		tenantID, "User "+email, email, string(hash)).Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

// APIClient is an HTTP client with a cookie jar that echoes the CSRF cookie
// into the X-CSRF-Token header on every request.
type APIClient struct {
	// Header holds extra headers sent on every request (e.g. X-Real-IP).
	Header http.Header

	t    *testing.T
	base string
	http *http.Client
	url  *url.URL
}

// Client returns a new APIClient for the server at srvURL.
func Client(t *testing.T, srvURL string) *APIClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(srvURL)
	if err != nil {
		t.Fatal(err)
	}
	return &APIClient{Header: http.Header{}, t: t, base: srvURL, http: &http.Client{Jar: jar}, url: u}
}

// Cookie returns the value of the named cookie in the jar, or "".
func (c *APIClient) Cookie(name string) string {
	for _, ck := range c.http.Jar.Cookies(c.url) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

func (c *APIClient) do(method, path string, body any, csrf bool) *http.Response {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	if csrf && c.Cookie("csrf_token") == "" {
		// Any response sets the CSRF cookie; prime the jar.
		res, err := c.http.Get(c.base + "/api/health")
		if err != nil {
			c.t.Fatal(err)
		}
		res.Body.Close()
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf {
		req.Header.Set("X-CSRF-Token", c.Cookie("csrf_token"))
	}
	for k, v := range c.Header {
		req.Header[k] = v
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}

// Do sends a request with the CSRF header set. The caller closes the body.
func (c *APIClient) Do(method, path string, body any) *http.Response {
	c.t.Helper()
	return c.do(method, path, body, true)
}

// DoWithoutCSRF sends a request without the X-CSRF-Token header.
func (c *APIClient) DoWithoutCSRF(method, path string, body any) *http.Response {
	c.t.Helper()
	return c.do(method, path, body, false)
}

// Login posts credentials and returns the response (body closed after read by JSON, or by caller).
func (c *APIClient) Login(email, pw string) *http.Response {
	c.t.Helper()
	return c.Do("POST", "/api/auth/login", map[string]string{"email": email, "password": pw})
}

// JSON decodes and closes the response body into v.
func (c *APIClient) JSON(res *http.Response, v any) {
	c.t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		c.t.Fatalf("decode response (status %d): %v", res.StatusCode, err)
	}
}

// DoRaw sends body as-is with the given Content-Type and the CSRF header set.
// The caller closes the response body.
func (c *APIClient) DoRaw(method, path string, body io.Reader, contentType string) *http.Response {
	c.t.Helper()
	if c.Cookie("csrf_token") == "" {
		res, err := c.http.Get(c.base + "/api/health")
		if err != nil {
			c.t.Fatal(err)
		}
		res.Body.Close()
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", c.Cookie("csrf_token"))
	for k, v := range c.Header {
		req.Header[k] = v
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}
