package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"assetku/api/internal/httpx"
)

func TestDecodeRejectsOversizedBody(t *testing.T) {
	big := `{"name":"` + strings.Repeat("a", httpx.MaxJSONBytes) + `"}`
	var v struct{ Name string }
	err := httpx.Decode(httptest.NewRequest("POST", "/", strings.NewReader(big)), &v)
	var he httpx.Error
	if !errors.As(err, &he) || he.Status != http.StatusRequestEntityTooLarge || he.Code != "body_too_large" {
		t.Fatalf("err %v", err)
	}

	ok := `{"name":"` + strings.Repeat("a", 1000) + `"}`
	if err := httpx.Decode(httptest.NewRequest("POST", "/", strings.NewReader(ok)), &v); err != nil || len(v.Name) != 1000 {
		t.Fatalf("small body: %v", err)
	}
}

func TestRealIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	cases := []struct {
		name, remote, realIP string
		trusted              []netip.Prefix
		want                 string
	}{
		{"trusted proxy", "172.18.0.5:41000", "203.0.113.7", trusted, "203.0.113.7:0"},
		{"untrusted peer spoofing", "198.51.100.9:5000", "203.0.113.7", trusted, "198.51.100.9:5000"},
		{"no proxies configured", "172.18.0.5:41000", "203.0.113.7", nil, "172.18.0.5:41000"},
		{"trusted proxy, header missing", "172.18.0.5:41000", "", trusted, "172.18.0.5:41000"},
		{"trusted proxy, garbage header", "172.18.0.5:41000", "not-an-ip", trusted, "172.18.0.5:41000"},
		{"ipv6 client", "172.18.0.5:41000", "2001:db8::1", trusted, "[2001:db8::1]:0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			h := httpx.RealIP(c.trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = c.remote
			if c.realIP != "" {
				req.Header.Set("X-Real-IP", c.realIP)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != c.want {
				t.Fatalf("RemoteAddr %q, want %q", got, c.want)
			}
		})
	}
}
