package httpx

import (
	"net/http"
	"net/netip"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds shared dependencies for handlers.
type Deps struct {
	Pool         *pgxpool.Pool
	CookieSecure bool
	// TrustedProxies are the peers whose X-Real-IP header is believed (see RealIP).
	TrustedProxies []netip.Prefix
	// Register mounts feature routes (e.g. auth.Service.Register) on the mux.
	Register func(mux *http.ServeMux)
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", H(func(w http.ResponseWriter, r *http.Request) error {
		JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return nil
	}))
	if deps.Register != nil {
		deps.Register(mux)
	}
	return RealIP(deps.TrustedProxies)(CSRF(deps.CookieSecure)(mux))
}
