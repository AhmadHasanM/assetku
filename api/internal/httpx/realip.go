package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// RealIP replaces r.RemoteAddr with the X-Real-IP header, but only when the
// direct peer is inside one of the trusted proxy prefixes (nginx in the compose
// stack). Anyone else could forge the header, so it is ignored for them, and
// with no trusted prefixes the header is never used. Login rate limiting keys on
// RemoteAddr, so this is what makes it per client behind the proxy.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(trusted) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client, ok := forwardedClient(r, trusted); ok {
				r2 := r.Clone(r.Context())
				r2.RemoteAddr = net.JoinHostPort(client.String(), "0")
				r = r2
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClient(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	hdr := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if hdr == "" {
		return netip.Addr{}, false
	}
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	if !inAny(peer.Addr().Unmap(), trusted) {
		return netip.Addr{}, false
	}
	client, err := netip.ParseAddr(hdr)
	if err != nil {
		return netip.Addr{}, false
	}
	return client.Unmap(), true
}

func inAny(a netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
