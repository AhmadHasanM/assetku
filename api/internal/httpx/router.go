package httpx

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds shared dependencies for handlers.
type Deps struct {
	Pool *pgxpool.Pool
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", H(func(w http.ResponseWriter, r *http.Request) error {
		JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return nil
	}))
	return mux
}
