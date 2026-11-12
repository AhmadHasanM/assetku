package asset

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"assetku/api/internal/httpx"
)

// ListItem is one row of the asset list.
type ListItem struct {
	ID               uuid.UUID `json:"id"`
	AssetTag         string    `json:"asset_tag"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Model            string    `json:"model"`
	SerialNumber     string    `json:"serial_number"`
	Status           string    `json:"status"`
	AssignedTo       string    `json:"assigned_to"`
	LocationName     string    `json:"location_name"`
	OrganizationName string    `json:"organization_name"`
}

// searchExpr must stay textually equivalent to the expression of the
// assets_search_trgm_idx GIN index (migration 0004) so the index is used.
const searchExpr = `lower(a.asset_tag || ' ' || a.name || ' ' || a.serial_number || ' ' || a.model || ' ' || a.assigned_to)`

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// where builds the WHERE clause (always tenant-scoped, $1) and its arguments.
func (f ListFilter) where(tenantID uuid.UUID) (string, []any) {
	args := []any{tenantID}
	conds := []string{"a.tenant_id = $1"}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		add(searchExpr+` LIKE '%%' || lower($%d::text) || '%%'`, likeEscaper.Replace(q))
	}
	if len(f.Categories) > 0 {
		add("a.category = ANY($%d::text[])", f.Categories)
	}
	if len(f.Statuses) > 0 {
		add("a.status = ANY($%d::text[])", f.Statuses)
	}
	if f.LocationID != nil {
		add("a.location_id = $%d", *f.LocationID)
	}
	if f.OrganizationID != nil {
		add("a.organization_id = $%d", *f.OrganizationID)
	}
	return strings.Join(conds, " AND "), args
}

// orderBy returns the ORDER BY expression list for f (whitelisted, never interpolated input).
func (f ListFilter) orderBy() string {
	col, ok := sortColumns[f.Sort]
	if !ok {
		col = sortColumns["created_at"]
	}
	dir := "DESC"
	if f.Dir == "asc" {
		dir = "ASC"
	}
	return col + " " + dir + ", a.id " + dir
}

// List returns one page (limit > 0, offset from f.Page) or all matching rows
// (limit <= 0) plus the total number of matches.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, f ListFilter, limit int) ([]ListItem, int, error) {
	where, args := f.where(tenantID)
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM assets a WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := f.orderBy()
	sql := `SELECT a.id, a.asset_tag, a.name, a.category, a.model, a.serial_number, a.status, a.assigned_to,
		COALESCE(l.name, ''), COALESCE(o.name, '')
		FROM assets a
		LEFT JOIN locations l ON l.id = a.location_id AND l.tenant_id = a.tenant_id
		LEFT JOIN organizations o ON o.id = a.organization_id AND o.tenant_id = a.tenant_id
		WHERE ` + where + " ORDER BY " + order
	if limit > 0 {
		page := max(f.Page, 1)
		args = append(args, limit, (page-1)*limit)
		sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []ListItem{}
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(&it.ID, &it.AssetTag, &it.Name, &it.Category, &it.Model, &it.SerialNumber,
			&it.Status, &it.AssignedTo, &it.LocationName, &it.OrganizationName); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// Summary holds tenant-wide counts, independent of any list filter.
type Summary struct {
	Total          int `json:"total"`
	Available      int `json:"available"`
	InUse          int `json:"in_use"`
	NeedsAttention int `json:"needs_attention"`
}

// Summary counts the tenant's assets; archived only counts toward total.
func (s *Service) Summary(ctx context.Context, tenantID uuid.UUID) (Summary, error) {
	var sum Summary
	err := s.pool.QueryRow(ctx, `SELECT count(*),
		count(*) FILTER (WHERE status = 'available'),
		count(*) FILTER (WHERE status = 'in_use'),
		count(*) FILTER (WHERE status IN ('maintenance', 'damaged', 'lost'))
		FROM assets WHERE tenant_id = $1`, tenantID).Scan(&sum.Total, &sum.Available, &sum.InUse, &sum.NeedsAttention)
	return sum, err
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) error {
	f, err := ParseListFilter(r.URL.Query())
	if err != nil {
		return err
	}
	items, total, err := s.List(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, f, PageSize)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": f.Page, "page_size": PageSize})
	return nil
}

func (s *Service) summary(w http.ResponseWriter, r *http.Request) error {
	sum, err := s.Summary(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, sum)
	return nil
}
