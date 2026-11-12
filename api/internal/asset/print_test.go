package asset_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"assetku/api/internal/testutil"
)

func readPDF(t *testing.T, res io.ReadCloser) []byte {
	t.Helper()
	defer res.Close()
	b, err := io.ReadAll(res)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLabelPDFAndHistory(t *testing.T) {
	e := setup(t)
	a := create(t, e.a, map[string]any{"name": "Laptop & \"Pro\" é", "category": "it_equipment", "status": "in_use", "asset_tag": "AST-8F2K1Q9Z", "assigned_to": "Budi"})
	res := e.a.Do("POST", "/api/assets/"+a.ID+"/label", nil)
	if res.StatusCode != 200 {
		t.Fatalf("label %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type %q", ct)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="label-AST-8F2K1Q9Z.pdf"`) {
		t.Fatalf("disposition %q", cd)
	}
	if b := readPDF(t, res.Body); !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatal("not a pdf")
	}
	got := getAsset(t, e.a, a.ID)
	if got.Status != "in_use" || got.AssignedTo != "Budi" {
		t.Fatalf("print changed asset: %+v", got)
	}
	if h := getHistory(t, e.a, a.ID); h.Items[0].Event != "label_printed" {
		t.Fatalf("history %+v", h.Items)
	}
}

func getAsset(t *testing.T, c *testutil.APIClient, id string) assetJSON {
	t.Helper()
	res := c.Do("GET", "/api/assets/"+id, nil)
	var a assetJSON
	c.JSON(res, &a)
	return a
}

func TestLabelCrossTenant404(t *testing.T) {
	e := setup(t)
	a := create(t, e.a, baseBody("Laptop"))
	for _, c := range []struct{ method, path string }{
		{"POST", "/label"}, {"GET", "/bast/defaults"}, {"POST", "/bast"},
	} {
		res := e.b.Do(c.method, "/api/assets/"+a.ID+c.path, map[string]any{"second_name": "X", "condition": "good"})
		if res.StatusCode != 404 {
			t.Fatalf("%s %s: %d, want 404", c.method, c.path, res.StatusCode)
		}
		res.Body.Close()
	}
	if n := len(getHistory(t, e.a, a.ID).Items); n != 1 {
		t.Fatalf("history entries = %d, want 1 (created only)", n)
	}
}

func TestBASTRequiresSecondName422(t *testing.T) {
	e := setup(t)
	a := create(t, e.a, baseBody("Laptop"))
	res := e.a.Do("POST", "/api/assets/"+a.ID+"/bast", map[string]any{"date": "2026-11-11", "condition": "good", "second_name": "  "})
	expect422(t, e.a, res, map[string]string{"second_name": "required"})
	res = e.a.Do("POST", "/api/assets/"+a.ID+"/bast", map[string]any{"date": "2026-11-11", "condition": "broken", "second_name": "Siti"})
	expect422(t, e.a, res, map[string]string{"condition": "invalid"})
	if n := len(getHistory(t, e.a, a.ID).Items); n != 1 {
		t.Fatalf("failed BAST must not record history, entries = %d", n)
	}
}

func TestBASTNumberAndHistory(t *testing.T) {
	e := setup(t)
	a := create(t, e.a, map[string]any{"name": "Laptop", "category": "it_equipment", "status": "available", "asset_tag": "AST-8F2K1Q9Z"})
	res := e.a.Do("POST", "/api/assets/"+a.ID+"/bast", map[string]any{
		"date": "2026-11-11", "first_name": "Budi", "first_position": "Manajer", "second_name": "Siti",
		"second_position": "Staf", "second_department": "Keuangan", "condition": "new_sealed", "warranty": "s.d. 11 November 2027", "notes": "ok",
	})
	if res.StatusCode != 200 {
		t.Fatalf("bast %d", res.StatusCode)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="BAST-AST-8F2K1Q9Z-20261111.pdf"`) {
		t.Fatalf("disposition %q", cd)
	}
	if b := readPDF(t, res.Body); !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatal("not a pdf")
	}
	h := getHistory(t, e.a, a.ID)
	if h.Items[0].Event != "bast_printed" || h.Items[0].Detail["number"] != "BAST-AST-8F2K1Q9Z-20261111" {
		t.Fatalf("history %+v", h.Items[0])
	}
	got := getAsset(t, e.a, a.ID)
	if got.Status != "available" || got.AssignedTo != "" {
		t.Fatalf("BAST changed asset: %+v", got)
	}
}

func TestBASTDefaults(t *testing.T) {
	e := setup(t)
	if _, err := e.pool.Exec(context.Background(), `UPDATE users SET name = 'Budi', position = 'Manajer' WHERE tenant_id = $1`, e.tenantA); err != nil {
		t.Fatal(err)
	}
	a := create(t, e.a, map[string]any{
		"name": "Laptop", "category": "it_equipment", "status": "in_use", "asset_tag": "AST-8F2K1Q9Z", "assigned_to": "Siti",
		"purchase_date": "2026-11-11", "warranty_months": 12,
	})
	res := e.a.Do("GET", "/api/assets/"+a.ID+"/bast/defaults", nil)
	if res.StatusCode != 200 {
		t.Fatalf("defaults %d", res.StatusCode)
	}
	var d map[string]string
	e.a.JSON(res, &d)
	if d["first_name"] != "Budi" || d["first_position"] != "Manajer" || d["second_name"] != "Siti" || d["warranty"] != "s.d. 11 November 2027" {
		t.Fatalf("defaults %+v", d)
	}
	if want := "BAST-AST-8F2K1Q9Z-" + strings.ReplaceAll(d["date"], "-", ""); d["number"] != want {
		t.Fatalf("number %q, want %q", d["number"], want)
	}
	// Without a warranty date the default is empty.
	b := create(t, e.a, baseBody("Meja"))
	res = e.a.Do("GET", "/api/assets/"+b.ID+"/bast/defaults", nil)
	e.a.JSON(res, &d)
	if d["warranty"] != "" {
		t.Fatalf("warranty %q, want empty", d["warranty"])
	}
}
