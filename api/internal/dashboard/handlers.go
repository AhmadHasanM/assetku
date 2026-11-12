// Package dashboard implements the tenant dashboard endpoint.
package dashboard

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
)

const (
	listLimit       = 50
	upcomingDays    = 30
	jakartaZoneName = "Asia/Jakarta"
)

// Service serves GET /api/dashboard.
type Service struct {
	pool   *pgxpool.Pool
	auth   *auth.Service
	assets *asset.Service
	// Today returns the current date in Asia/Jakarta; tests replace it.
	Today func() civil.Date
}

func NewService(pool *pgxpool.Pool, a *auth.Service, assets *asset.Service) *Service {
	return &Service{pool: pool, auth: a, assets: assets, Today: jakartaToday}
}

func jakartaToday() civil.Date {
	loc, err := time.LoadLocation(jakartaZoneName)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return civil.DateOf(time.Now().In(loc))
}

// Register mounts the dashboard route; it requires a user session.
func (s *Service) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/dashboard", httpx.RequireUser(s.auth)(httpx.H(s.get)))
}

type AttentionItem struct {
	ID           uuid.UUID `json:"id"`
	AssetTag     string    `json:"asset_tag"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	LocationName *string   `json:"location_name"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpcomingItem struct {
	AssetID  uuid.UUID  `json:"asset_id"`
	AssetTag string     `json:"asset_tag"`
	Name     string     `json:"name"`
	NextDate civil.Date `json:"next_date"`
	Overdue  bool       `json:"overdue"`
}

type KeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type Dashboard struct {
	Summary             asset.Summary   `json:"summary"`
	Attention           []AttentionItem `json:"attention"`
	UpcomingMaintenance []UpcomingItem  `json:"upcoming_maintenance"`
	ByCategory          []KeyCount      `json:"by_category"`
	ByStatus            []KeyCount      `json:"by_status"`
}

// Get assembles the dashboard of one tenant.
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID) (Dashboard, error) {
	var d Dashboard
	var err error
	if d.Summary, err = s.assets.Summary(ctx, tenantID); err != nil {
		return d, err
	}
	if d.Attention, err = s.attention(ctx, tenantID); err != nil {
		return d, err
	}
	if d.UpcomingMaintenance, err = s.upcoming(ctx, tenantID, s.Today()); err != nil {
		return d, err
	}
	if d.ByCategory, err = s.counts(ctx, tenantID, "category", asset.Categories); err != nil {
		return d, err
	}
	d.ByStatus, err = s.counts(ctx, tenantID, "status", asset.Statuses)
	return d, err
}

func (s *Service) attention(ctx context.Context, tenantID uuid.UUID) ([]AttentionItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.asset_tag, a.name, a.status, l.name, a.updated_at
		FROM assets a
		LEFT JOIN locations l ON l.id = a.location_id AND l.tenant_id = a.tenant_id
		WHERE a.tenant_id = $1 AND a.status IN ('maintenance', 'damaged', 'lost')
		ORDER BY a.updated_at DESC, a.id DESC
		LIMIT $2`, tenantID, listLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AttentionItem{}
	for rows.Next() {
		var it AttentionItem
		if err := rows.Scan(&it.ID, &it.AssetTag, &it.Name, &it.Status, &it.LocationName, &it.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// upcoming applies the shared latest-record rule first (one record per asset),
// then keeps those whose next_date is set and within today+30 (overdue included).
func (s *Service) upcoming(ctx context.Context, tenantID uuid.UUID, today civil.Date) ([]UpcomingItem, error) {
	limit := today.AddDays(upcomingDays)
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.asset_tag, a.name, m.next_date
		FROM (
			SELECT DISTINCT ON (asset_id) asset_id, next_date
			FROM maintenance_records
			WHERE tenant_id = $1
			ORDER BY asset_id, date DESC, created_at DESC, id DESC
		) m
		JOIN assets a ON a.id = m.asset_id AND a.tenant_id = $1
		WHERE m.next_date IS NOT NULL AND m.next_date <= $2::date
		ORDER BY m.next_date ASC, a.id ASC
		LIMIT $3`, tenantID, limit.String(), listLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UpcomingItem{}
	for rows.Next() {
		var it UpcomingItem
		var next time.Time
		if err := rows.Scan(&it.AssetID, &it.AssetTag, &it.Name, &next); err != nil {
			return nil, err
		}
		it.NextDate = civil.DateOf(next)
		it.Overdue = it.NextDate.Before(today)
		out = append(out, it)
	}
	return out, rows.Err()
}

// counts returns one entry per key in canonical order, zero counts included.
func (s *Service) counts(ctx context.Context, tenantID uuid.UUID, column string, keys []string) ([]KeyCount, error) {
	// column is one of two package-internal literals, never user input.
	rows, err := s.pool.Query(ctx, `SELECT `+column+`, count(*) FROM assets WHERE tenant_id = $1 GROUP BY 1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		got[k] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]KeyCount, 0, len(keys))
	for _, k := range keys {
		out = append(out, KeyCount{Key: k, Count: got[k]})
	}
	return out, nil
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) error {
	d, err := s.Get(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, d)
	return nil
}
