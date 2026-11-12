package export

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/importer"
)

// Service serves GET /api/assets/export. At most asset.ExportMaxRows (50,000)
// rows are exported; larger result sets get 422 export_too_large.
type Service struct {
	pool   *pgxpool.Pool
	auth   *auth.Service
	assets *asset.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service, assets *asset.Service) *Service {
	return &Service{pool: pool, auth: a, assets: assets}
}

// Register mounts the route. The literal path is more specific than
// /api/assets/{id}, so ServeMux routes it here.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("GET /api/assets/export", user(httpx.H(s.export)))
}

var (
	errFormat   = httpx.Error{Status: http.StatusBadRequest, Code: "invalid_format", Message: "format must be csv, xlsx or pdf"}
	errTooLarge = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "export_too_large",
		Message: fmt.Sprintf("more than %d rows match; narrow the filter", asset.ExportMaxRows)}
)

var contentTypes = map[string]string{
	"csv":  "text/csv; charset=utf-8",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"pdf":  "application/pdf",
}

func (s *Service) refName(ctx context.Context, table string, tenantID uuid.UUID, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	var name string
	// table is one of two constants below, never user input.
	_ = s.pool.QueryRow(ctx, "SELECT name FROM "+table+" WHERE id = $1 AND tenant_id = $2", *id, tenantID).Scan(&name)
	return name
}

func (s *Service) export(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	format := q.Get("format")
	if _, ok := contentTypes[format]; !ok {
		return errFormat
	}
	f, err := asset.ParseListFilter(q)
	if err != nil {
		return err
	}
	f.Page = 1
	p := httpx.PrincipalFrom(r.Context())
	rows, err := s.assets.ListForExport(r.Context(), p.TenantID, f)
	if err == asset.ErrExportTooLarge {
		return errTooLarge
	}
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	switch format {
	case "csv":
		err = WriteCSV(&buf, importer.CanonicalHeaders, rows)
	case "xlsx":
		err = WriteXLSX(&buf, importer.CanonicalHeaders, rows)
	case "pdf":
		var tenant string
		if err = s.pool.QueryRow(r.Context(), `SELECT name FROM tenants WHERE id = $1`, p.TenantID).Scan(&tenant); err != nil {
			return err
		}
		now := time.Now().In(asset.Jakarta)
		months := monthsID
		if p.Locale == "en" {
			months = monthsEN
		}
		printed := fmt.Sprintf("%02d %s %d %02d:%02d", now.Day(), months[now.Month()-1], now.Year(), now.Hour(), now.Minute())
		summary := filterSummary(f, s.refName(r.Context(), "locations", p.TenantID, f.LocationID),
			s.refName(r.Context(), "organizations", p.TenantID, f.OrganizationID), p.Locale)
		err = WritePDF(&buf, tenant, printed, summary, p.Locale, rows)
	}
	if err != nil {
		return err
	}
	name := "assets-" + time.Now().In(asset.Jakarta).Format("20060102") + "." + format
	h := w.Header()
	h.Set("Content-Type", contentTypes[format])
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	h.Set("Content-Length", fmt.Sprint(buf.Len()))
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
	return nil
}
