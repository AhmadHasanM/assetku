package asset

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/pdf"
)

// PrintService renders the asset label and BAST documents. Printing never
// changes the asset; it only records a history entry.
type PrintService struct {
	pool       *pgxpool.Pool
	auth       *auth.Service
	assets     *Service
	appBaseURL string
}

func NewPrintService(pool *pgxpool.Pool, a *auth.Service, assets *Service, appBaseURL string) *PrintService {
	return &PrintService{pool: pool, auth: a, assets: assets, appBaseURL: strings.TrimRight(appBaseURL, "/")}
}

// Register mounts the print routes; all require a user session.
func (s *PrintService) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("POST /api/assets/{id}/label", user(httpx.H(s.label)))
	mux.Handle("GET /api/assets/{id}/bast/defaults", user(httpx.H(s.bastDefaults)))
	mux.Handle("POST /api/assets/{id}/bast", user(httpx.H(s.bast)))
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func bastNumber(tag string, d civil.Date) string {
	return fmt.Sprintf("BAST-%s-%04d%02d%02d", tag, d.Year, int(d.Month), d.Day)
}

func (s *PrintService) tenantName(ctx context.Context, tenantID uuid.UUID) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `SELECT name FROM tenants WHERE id = $1`, tenantID).Scan(&name)
	return name, err
}

func sendPDF(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, unsafeFileChars.ReplaceAllString(filename, "_")))
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *PrintService) label(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(r.Context())
	a, err := s.assets.Get(r.Context(), p.TenantID, id)
	if err != nil {
		return apiErr(err)
	}
	tenant, err := s.tenantName(r.Context(), p.TenantID)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = pdf.Label(&buf, pdf.LabelData{
		QRURL: s.appBaseURL + "/assets/" + a.ID.String(), Tag: a.AssetTag, Name: a.Name,
		Model: a.Model, Serial: a.SerialNumber, TenantName: tenant,
	})
	if err != nil {
		return err
	}
	if err := s.record(r.Context(), p, a.ID, history.EventLabelPrinted, nil); err != nil {
		return err
	}
	sendPDF(w, "label-"+a.AssetTag, buf.Bytes())
	return nil
}

func (s *PrintService) record(ctx context.Context, p httpx.Principal, assetID uuid.UUID, event string, detail map[string]any) error {
	uid := p.UserID
	return history.Record(ctx, s.pool, history.Entry{
		TenantID: p.TenantID, AssetID: assetID, Event: event, ActorID: &uid, ActorName: p.Name, Detail: detail,
	})
}

func (s *PrintService) bastDefaults(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(r.Context())
	a, err := s.assets.Get(r.Context(), p.TenantID, id)
	if err != nil {
		return apiErr(err)
	}
	today := Today()
	warranty := ""
	if a.WarrantyExpires != nil {
		warranty = "s.d. " + pdf.FormatDateID(*a.WarrantyExpires)
	}
	httpx.JSON(w, http.StatusOK, map[string]string{
		"number":         bastNumber(a.AssetTag, today),
		"date":           today.String(),
		"first_name":     p.Name,
		"first_position": p.Position,
		"second_name":    a.AssignedTo,
		"warranty":       warranty,
	})
	return nil
}

type bastRequest struct {
	Date             *civil.Date `json:"date"`
	FirstName        string      `json:"first_name"`
	FirstPosition    string      `json:"first_position"`
	SecondName       string      `json:"second_name"`
	SecondPosition   string      `json:"second_position"`
	SecondDepartment string      `json:"second_department"`
	Condition        string      `json:"condition"`
	Warranty         string      `json:"warranty"`
	Notes            string      `json:"notes"`
}

func (s *PrintService) bast(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in bastRequest
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	p := httpx.PrincipalFrom(r.Context())
	a, err := s.assets.Get(r.Context(), p.TenantID, id)
	if err != nil {
		return apiErr(err)
	}

	fields := map[string]string{}
	if strings.TrimSpace(in.SecondName) == "" {
		fields["second_name"] = "required"
	}
	condition, ok := pdf.ConditionLabel(in.Condition)
	if !ok {
		fields["condition"] = "invalid"
	}
	if len(fields) > 0 {
		return apiErr(&ValidationError{Fields: fields})
	}
	date := Today()
	if in.Date != nil {
		date = *in.Date
	}
	if !date.IsValid() || date.Year < 1900 || date.Year > 2200 {
		return apiErr(&ValidationError{Fields: map[string]string{"date": "invalid"}})
	}

	tenant, err := s.tenantName(r.Context(), p.TenantID)
	if err != nil {
		return err
	}
	number := bastNumber(a.AssetTag, date)
	var buf bytes.Buffer
	err = pdf.BAST(&buf, pdf.BASTData{
		Number: number, TenantName: tenant, Date: date,
		FirstName: in.FirstName, FirstPosition: in.FirstPosition,
		SecondName: in.SecondName, SecondPosition: in.SecondPosition, SecondDepartment: in.SecondDepartment,
		Tag: a.AssetTag, AssetName: a.Name, Category: pdf.CategoryLabel(a.Category),
		BrandModel: strings.TrimSpace(a.Manufacturer + " " + a.Model), Serial: a.SerialNumber,
		Condition: condition, Warranty: in.Warranty, Notes: in.Notes,
	})
	if err != nil {
		return err
	}
	if err := s.record(r.Context(), p, a.ID, history.EventBastPrinted, map[string]any{"number": number}); err != nil {
		return err
	}
	sendPDF(w, number, buf.Bytes())
	return nil
}
