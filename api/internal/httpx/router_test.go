package httpx_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"assetku/api/internal/httpx"
)

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("got %d %s", res.StatusCode, body)
	}
}

func TestErrorFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.H(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.Error{Status: 422, Code: "validation", Message: "invalid", Fields: map[string]string{"name": "required"}}
	})(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 422 {
		t.Fatalf("status %d", rec.Code)
	}
	var got struct {
		Error struct {
			Code, Message string
			Fields        map[string]string
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "validation" || got.Error.Message != "invalid" || got.Error.Fields["name"] != "required" {
		t.Fatalf("got %+v", got)
	}

	rec = httptest.NewRecorder()
	httpx.H(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("boom")
	})(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"code":"internal"`) || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("{"))
	var v map[string]any
	var he httpx.Error
	if err := httpx.Decode(r, &v); !errors.As(err, &he) || he.Status != 400 || he.Code != "invalid_json" {
		t.Fatalf("got %v", err)
	}
}
