package history

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/httpx"
)

// PageSize is the number of history entries per page.
const PageSize = 20

var errNotFound = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}

// Service serves GET /api/assets/{id}/history.
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service {
	return &Service{pool: pool, auth: a}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/assets/{id}/history", httpx.RequireUser(s.auth)(httpx.H(s.list)))
}

type item struct {
	ID        int64           `json:"id"`
	Event     string          `json:"event"`
	ActorName string          `json:"actor_name"`
	Detail    json.RawMessage `json:"detail"`
	CreatedAt time.Time       `json:"created_at"`
}

type page struct {
	Items      []item `json:"items"`
	NextBefore *int64 `json:"next_before"`
}

// list returns the asset's history newest first, PageSize per page; pass
// next_before as ?before= for the next page (null on the last page).
func (s *Service) list(w http.ResponseWriter, r *http.Request) error {
	assetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return errNotFound
	}
	before := pgtype.Int8{}
	if v := r.URL.Query().Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "validation failed",
				Fields: map[string]string{"before": "invalid"}}
		}
		before = pgtype.Int8{Int64: n, Valid: true}
	}
	tenantID := httpx.PrincipalFrom(r.Context()).TenantID
	q := gen.New(s.pool)
	ok, err := q.AssetExists(r.Context(), gen.AssetExistsParams{TenantID: tenantID, ID: assetID})
	if err != nil {
		return err
	}
	if !ok {
		return errNotFound
	}
	rows, err := q.ListHistory(r.Context(), gen.ListHistoryParams{TenantID: tenantID, AssetID: assetID, Before: before, Lim: PageSize + 1})
	if err != nil {
		return err
	}
	out := page{Items: []item{}}
	for i, row := range rows {
		if i == PageSize {
			last := out.Items[PageSize-1].ID
			out.NextBefore = &last
			break
		}
		out.Items = append(out.Items, item{row.ID, row.Event, row.ActorName, row.Detail, row.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}
