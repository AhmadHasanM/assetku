package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/config"
	"assetku/api/internal/httpx"
)

// multipartOverhead is the allowance for multipart framing on top of
// MaxFileBytes when limiting the request body.
const multipartOverhead = 64 << 10

var (
	errNotFound    = httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}
	errTooLarge    = httpx.Error{Status: http.StatusRequestEntityTooLarge, Code: "file_too_large", Message: "file exceeds 10 MB"}
	errUnsupported = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "unsupported_format", Message: "file must be csv, xlsx or xls"}
	errNoHeader    = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "header_not_found", Message: "no recognized header in the first 10 rows"}
	errTooMany     = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "too_many_rows", Message: "file has more than 10000 data rows"}
	errUnreadable  = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "invalid_file", Message: "file could not be read"}
	errFileMissing = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: "file is required",
		Fields: map[string]string{"file": "required"}}
	errNotFinished = httpx.Error{Status: http.StatusConflict, Code: "import_not_finished", Message: "import has not finished"}
)

// Service serves the import endpoints and runs import jobs in background
// goroutines (not tied to the request context).
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
	wg   sync.WaitGroup
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service {
	return &Service{pool: pool, auth: a}
}

// Wait blocks until every job started by this service has finished.
func (s *Service) Wait() { s.wg.Wait() }

// Register mounts the import routes; all require a user session. The literal
// template.xlsx route is more specific than {id}, so ServeMux prefers it.
func (s *Service) Register(mux *http.ServeMux) {
	user := httpx.RequireUser(s.auth)
	mux.Handle("POST /api/imports", user(httpx.H(s.upload)))
	mux.Handle("GET /api/imports/template.xlsx", user(httpx.H(s.template)))
	mux.Handle("GET /api/imports/{id}", user(httpx.H(s.get)))
	mux.Handle("GET /api/imports/{id}/report.csv", user(httpx.H(s.report)))
}

// readUpload streams the multipart body and returns the "file" part's name
// and content. The body is capped before any multipart parsing.
func readUpload(w http.ResponseWriter, r *http.Request) (string, []byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileBytes+multipartOverhead)
	mr, err := r.MultipartReader()
	if err != nil {
		return "", nil, errFileMissing
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return "", nil, errTooLarge
			}
			return "", nil, errFileMissing // io.EOF or malformed body
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		name := part.FileName()
		if !supportedExt(name) {
			return "", nil, errUnsupported
		}
		data, err := io.ReadAll(io.LimitReader(part, MaxFileBytes+1))
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return "", nil, errTooLarge
			}
			return "", nil, errUnreadable
		}
		if len(data) > MaxFileBytes {
			return "", nil, errTooLarge
		}
		return name, data, nil
	}
}

func supportedExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv", ".xlsx", ".xls":
		return true
	}
	return false
}

func (s *Service) upload(w http.ResponseWriter, r *http.Request) error {
	p := httpx.PrincipalFrom(r.Context())
	name, data, err := readUpload(w, r)
	if err != nil {
		return err
	}
	f, err := parseFile(name, bytes.NewReader(data))
	switch {
	case errors.Is(err, ErrUnsupportedFormat):
		return errUnsupported
	case errors.Is(err, errHeaderNotFound):
		return errNoHeader
	case errors.Is(err, errTooManyRows):
		return errTooMany
	case err != nil:
		return errUnreadable
	}
	var id uuid.UUID
	err = s.pool.QueryRow(r.Context(), `INSERT INTO imports (tenant_id, user_id, file_name, total_rows)
		VALUES ($1, $2, $3, $4) RETURNING id`, p.TenantID, p.UserID, filepath.Base(name), len(f.rows)).Scan(&id)
	if err != nil {
		return err
	}
	putJob(id, &job{principal: p, file: f})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := Run(context.Background(), s.pool, id); err != nil {
			log.Printf("import %s failed: %v", id, err)
		}
	}()
	httpx.JSON(w, http.StatusAccepted, map[string]any{"id": id})
	return nil
}

type importView struct {
	ID            uuid.UUID       `json:"id"`
	Status        string          `json:"status"`
	TotalRows     int             `json:"total_rows"`
	ProcessedRows int             `json:"processed_rows"`
	Result        json.RawMessage `json:"result"`
	Error         string          `json:"error"`
}

// load returns the tenant's import or errNotFound.
func (s *Service) load(ctx context.Context, tenantID uuid.UUID, rawID string) (importView, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return importView{}, errNotFound
	}
	v := importView{ID: id}
	var result []byte
	err = s.pool.QueryRow(ctx, `SELECT status, total_rows, processed_rows, result, error
		FROM imports WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&v.Status, &v.TotalRows, &v.ProcessedRows, &result, &v.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return importView{}, errNotFound
	}
	if err != nil {
		return importView{}, err
	}
	if result != nil {
		v.Result = result
	} else {
		v.Result = json.RawMessage("null")
	}
	return v, nil
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) error {
	v, err := s.load(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, v)
	return nil
}

func (s *Service) report(w http.ResponseWriter, r *http.Request) error {
	v, err := s.load(r.Context(), httpx.PrincipalFrom(r.Context()).TenantID, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.Status != "done" {
		return errNotFinished
	}
	res := NewResult()
	if err := json.Unmarshal(v.Result, res); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf") // UTF-8 BOM so Excel shows non-ASCII names correctly
	if err := WriteReport(&buf, res); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="import-report-%s.csv"`, v.ID))
	_, err = w.Write(buf.Bytes())
	return err
}

func (s *Service) template(w http.ResponseWriter, r *http.Request) error {
	var buf bytes.Buffer
	if err := WriteTemplate(&buf); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s-import-template.xlsx"`, strings.ToLower(config.ProductName)))
	_, err := w.Write(buf.Bytes())
	return err
}
