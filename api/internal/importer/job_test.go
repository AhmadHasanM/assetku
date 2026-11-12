package importer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"assetku/api/internal/auth"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

type httpEnv struct {
	pool *pgxpool.Pool
	svc  *Service
	a, b *testutil.APIClient
}

func setupHTTP(t *testing.T) *httpEnv {
	t.Helper()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	svc := NewService(pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		svc.Register(m)
	}}))
	// Cleanups run LIFO: wait for jobs before the server and pool go away.
	t.Cleanup(srv.Close)
	t.Cleanup(svc.Wait)
	e := &httpEnv{pool: pool, svc: svc}
	ta := testutil.CreateTenant(t, pool, "A")
	tb := testutil.CreateTenant(t, pool, "B")
	testutil.CreateUser(t, pool, ta, "a@a.test", "password-1")
	testutil.CreateUser(t, pool, tb, "b@b.test", "password-1")
	e.a, e.b = testutil.Client(t, srv.URL), testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{e.a: "a@a.test", e.b: "b@b.test"} {
		res := c.Login(em, "password-1")
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("login %s: %d", em, res.StatusCode)
		}
	}
	return e
}

func upload(t *testing.T, c *testutil.APIClient, name string, data []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	return c.DoRaw("POST", "/api/imports", &buf, mw.FormDataContentType())
}

type errBody struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func expectError(t *testing.T, c *testutil.APIClient, res *http.Response, status int, code string) {
	t.Helper()
	var e errBody
	c.JSON(res, &e)
	if res.StatusCode != status || e.Error.Code != code {
		t.Errorf("got %d %q, want %d %q", res.StatusCode, e.Error.Code, status, code)
	}
}

type jobView struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	TotalRows     int     `json:"total_rows"`
	ProcessedRows int     `json:"processed_rows"`
	Result        *Result `json:"result"`
	Error         string  `json:"error"`
}

func startImport(t *testing.T, c *testutil.APIClient, name, content string) string {
	t.Helper()
	res := upload(t, c, name, []byte(content))
	var out struct{ ID string }
	c.JSON(res, &out)
	if res.StatusCode != http.StatusAccepted || out.ID == "" {
		t.Fatalf("upload status=%d id=%q", res.StatusCode, out.ID)
	}
	return out.ID
}

func waitDone(t *testing.T, c *testutil.APIClient, id string) jobView {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		var v jobView
		res := c.Do("GET", "/api/imports/"+id, nil)
		c.JSON(res, &v)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("poll status %d", res.StatusCode)
		}
		if v.Status == "done" || v.Status == "failed" {
			return v
		}
		if v.ProcessedRows > v.TotalRows {
			t.Fatalf("processed %d > total %d", v.ProcessedRows, v.TotalRows)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job stuck: %+v", v)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestJobProgressReachesDone(t *testing.T) {
	e := setupHTTP(t)
	var sb strings.Builder
	sb.WriteString("Asset Name,Asset Tag,Status,Warna\n")
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&sb, "Item %d,J-%d,Tersedia,merah\n", i, i)
	}
	sb.WriteString(",,,\n")              // blank: not counted
	sb.WriteString("Untagged item,,,\n") // created with a generated tag
	sb.WriteString(",J-99,,\n")          // missing_name
	id := startImport(t, e.a, "items.csv", sb.String())
	v := waitDone(t, e.a, id)
	if v.Status != "done" || v.TotalRows != 62 || v.ProcessedRows != v.TotalRows || v.Error != "" {
		t.Fatalf("job=%+v", v)
	}
	r := v.Result
	if r.Created != 61 || r.Skipped != 1 || len(r.UnknownColumns) != 1 || r.UnknownColumns[0] != "Warna" {
		t.Errorf("result=%+v", r)
	}

	res := e.a.Do("GET", "/api/imports/"+id+"/report.csv", nil)
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") ||
		!strings.Contains(string(body), "row,reason\n64,missing_name\n") {
		t.Errorf("report %d %q:\n%s", res.StatusCode, res.Header.Get("Content-Disposition"), body)
	}

	res = e.a.Do("GET", "/api/imports/template.xlsx", nil)
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("template %d %q", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, _ := f.GetRows(f.GetSheetList()[0])
	if len(f.GetSheetList()) != 1 || len(rows) != 2 || strings.Join(rows[0], ",") != strings.Join(CanonicalHeaders, ",") {
		t.Errorf("template sheets=%v rows=%q", f.GetSheetList(), rows)
	}
}

func TestImportCrossTenantStatus404(t *testing.T) {
	e := setupHTTP(t)
	id := startImport(t, e.a, "a.csv", "Name\nLaptop\n")
	waitDone(t, e.a, id)
	for _, path := range []string{"/api/imports/" + id, "/api/imports/" + id + "/report.csv",
		"/api/imports/" + uuid.NewString(), "/api/imports/not-a-uuid"} {
		expectError(t, e.b, e.b.Do("GET", path, nil), http.StatusNotFound, "not_found")
	}
}

func TestUploadLimits(t *testing.T) {
	e := setupHTTP(t)
	big := bytes.Repeat([]byte("Name\nx\n"), (11<<20)/7)
	expectError(t, e.a, upload(t, e.a, "big.csv", big), http.StatusRequestEntityTooLarge, "file_too_large")

	var sb strings.Builder
	sb.WriteString("Name\n")
	for i := 0; i < MaxDataRows+1; i++ {
		sb.WriteString("x\n")
	}
	expectError(t, e.a, upload(t, e.a, "many.csv", []byte(sb.String())), http.StatusUnprocessableEntity, "too_many_rows")

	// An endless CSV is rejected without reading it all.
	if _, err := parseFile("endless.csv", &endlessCSV{}); !errors.Is(err, ErrTooManyRows) {
		t.Errorf("endless: %v", err)
	}
	// Exactly the limit, plus blank rows that do not count, parses.
	exact := strings.Replace(sb.String(), "Name\nx\n", "Name\n\n,\n", 1)
	if f, err := parseFile("exact.csv", strings.NewReader(exact)); err != nil || len(f.rows) != MaxDataRows {
		t.Errorf("exact limit: err=%v", err)
	}

	expectError(t, e.a, upload(t, e.a, "doc.pdf", []byte("%PDF-1.4")), http.StatusUnprocessableEntity, "unsupported_format")
	expectError(t, e.a, upload(t, e.a, "x.csv", []byte("foo,bar\n1,2\n")), http.StatusUnprocessableEntity, "header_not_found")

	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM imports`).Scan(&n)
	if n != 0 {
		t.Errorf("imports rows=%d, want 0", n)
	}
}

func TestFailInterrupted(t *testing.T) {
	e := setupHTTP(t)
	ctx := context.Background()
	var tenant uuid.UUID
	_ = e.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE name = 'A'`).Scan(&tenant)
	for _, st := range []string{"queued", "running", "done"} {
		if _, err := e.pool.Exec(ctx, `INSERT INTO imports (tenant_id, file_name, status) VALUES ($1, 'f.csv', $2)`, tenant, st); err != nil {
			t.Fatal(err)
		}
	}
	n, err := FailInterrupted(ctx, e.pool)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var failed int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM imports WHERE status = 'failed' AND error = 'server_restarted'`).Scan(&failed)
	if failed != 2 {
		t.Errorf("failed=%d", failed)
	}
	// A job whose in-memory rows are gone fails instead of hanging.
	var id uuid.UUID
	_ = e.pool.QueryRow(ctx, `INSERT INTO imports (tenant_id, file_name) VALUES ($1, 'g.csv') RETURNING id`, tenant).Scan(&id)
	if err := Run(ctx, e.pool, id); err == nil {
		t.Error("Run without job data succeeded")
	}
	v := waitDone(t, e.a, id.String())
	if v.Status != "failed" || v.Error != "job_data_missing" {
		t.Errorf("job=%+v", v)
	}
	expectError(t, e.a, e.a.Do("GET", "/api/imports/"+id.String()+"/report.csv", nil), http.StatusConflict, "import_not_finished")
}

func TestRunUnexpectedErrorStoresStableCode(t *testing.T) {
	e := setupHTTP(t)
	ctx := context.Background()
	var tenant, id uuid.UUID
	_ = e.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE name = 'A'`).Scan(&tenant)
	_ = e.pool.QueryRow(ctx, `INSERT INTO imports (tenant_id, file_name) VALUES ($1, 'h.csv') RETURNING id`, tenant).Scan(&id)
	putJob(id, &job{})
	cancelled, cancel := context.WithCancel(ctx)
	cancel() // the first DB call fails with a raw driver error
	if err := Run(cancelled, e.pool, id); err == nil {
		t.Fatal("Run with cancelled context succeeded")
	}
	v := waitDone(t, e.a, id.String())
	if v.Status != "failed" || v.Error != "internal" {
		t.Errorf("job=%+v", v)
	}
}
