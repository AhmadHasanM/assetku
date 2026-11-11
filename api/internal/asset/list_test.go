package asset_test

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"assetku/api/internal/asset"
	"assetku/api/internal/testutil"
)

type listResp struct {
	Items []struct {
		ID               string `json:"id"`
		AssetTag         string `json:"asset_tag"`
		Name             string `json:"name"`
		Category         string `json:"category"`
		Model            string `json:"model"`
		SerialNumber     string `json:"serial_number"`
		Status           string `json:"status"`
		AssignedTo       string `json:"assigned_to"`
		LocationName     string `json:"location_name"`
		OrganizationName string `json:"organization_name"`
	} `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func list(t *testing.T, c *testutil.APIClient, query string) listResp {
	t.Helper()
	res := c.Do("GET", "/api/assets?"+query, nil)
	if res.StatusCode != 200 {
		res.Body.Close()
		t.Fatalf("list %q: %d", query, res.StatusCode)
	}
	var l listResp
	c.JSON(res, &l)
	return l
}

func names(l listResp) []string {
	out := []string{}
	for _, it := range l.Items {
		out = append(out, it.Name)
	}
	return out
}

func sorted(s []string) []string { s = slices.Clone(s); slices.Sort(s); return s }

func TestSearchMatchesTagNameSerialModelAssignee(t *testing.T) {
	e := setup(t)
	b := baseBody("Laptop Zeta")
	b["asset_tag"] = "AST-TAGONE01"
	b["serial_number"] = "SN-Qwerty99"
	b["model"] = "ThinkPad X1"
	b["assigned_to"] = "Budi Santoso"
	create(t, e.a, b)
	create(t, e.a, baseBody("Other"))
	for _, q := range []string{"tagone", "TAGONE01", "zeta", "qwerty", "thinkpad", "budi san", "  budi  "} {
		l := list(t, e.a, "q="+url.QueryEscape(q))
		if l.Total != 1 || l.Items[0].Name != "Laptop Zeta" {
			t.Fatalf("q=%q total %d %v", q, l.Total, names(l))
		}
	}
	if l := list(t, e.a, "q=nomatchhere"); l.Total != 0 || len(l.Items) != 0 {
		t.Fatalf("nomatch %+v", l)
	}
	if l := list(t, e.a, "q=%20%20"); l.Total != 2 {
		t.Fatalf("blank q should not filter: %d", l.Total)
	}
}

func TestSearchEscapesWildcards(t *testing.T) {
	e := setup(t)
	create(t, e.a, baseBody("100% Cotton"))
	create(t, e.a, baseBody("Plain"))
	create(t, e.a, baseBody("under_score"))
	if l := list(t, e.a, "q=%25"); l.Total != 1 || l.Items[0].Name != "100% Cotton" {
		t.Fatalf("%% %v", names(l))
	}
	if l := list(t, e.a, "q=_"); l.Total != 1 || l.Items[0].Name != "under_score" {
		t.Fatalf("_ %v", names(l))
	}
	if l := list(t, e.a, "q="+url.QueryEscape(`\`)); l.Total != 0 {
		t.Fatalf("backslash %v", names(l))
	}
}

func TestFilterMultiStatusAndCategory(t *testing.T) {
	e := setup(t)
	mk := func(name, cat, st string) {
		b := baseBody(name)
		b["category"], b["status"] = cat, st
		create(t, e.a, b)
	}
	mk("a", "it_equipment", "available")
	mk("b", "vehicle", "in_use")
	mk("c", "vehicle", "damaged")
	mk("d", "furniture", "in_use")
	check := func(q string, want ...string) {
		t.Helper()
		if got := sorted(names(list(t, e.a, q))); !slices.Equal(got, want) {
			t.Fatalf("%s: got %v want %v", q, got, want)
		}
	}
	check("status=in_use,damaged", "b", "c", "d")
	check("category=vehicle,furniture", "b", "c", "d")
	check("category=vehicle&status=in_use,damaged", "b", "c")
	check("category=vehicle&status=available")
	check("status=", "a", "b", "c", "d")
}

func TestFilterLocationOrganization(t *testing.T) {
	e := setup(t)
	l1, l2 := masterID(t, e.a, "locations", "Gudang"), masterID(t, e.a, "locations", "Kantor")
	o1 := masterID(t, e.a, "organizations", "Finance")
	mk := func(name, loc, org string) {
		b := baseBody(name)
		if loc != "" {
			b["location_id"] = loc
		}
		if org != "" {
			b["organization_id"] = org
		}
		create(t, e.a, b)
	}
	mk("x", l1, o1)
	mk("y", l2, "")
	mk("z", "", "")
	if got := names(list(t, e.a, "location_id="+l1)); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("loc %v", got)
	}
	if got := names(list(t, e.a, "organization_id="+o1)); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("org %v", got)
	}
	if got := names(list(t, e.a, "location_id="+l2+"&organization_id="+o1)); len(got) != 0 {
		t.Fatalf("both %v", got)
	}
	l := list(t, e.a, "location_id="+l2)
	if l.Items[0].LocationName != "Kantor" || l.Items[0].OrganizationName != "" {
		t.Fatalf("names %+v", l.Items[0])
	}
	l = list(t, e.a, "location_id="+l1)
	if l.Items[0].LocationName != "Gudang" || l.Items[0].OrganizationName != "Finance" {
		t.Fatalf("names %+v", l.Items[0])
	}
}

func TestSortEachColumnBothDirs(t *testing.T) {
	e := setup(t)
	// Values are ordered opposite to creation order so sorting is observable.
	rows := []struct{ name, tag, cat, model, serial, status string }{
		{"n3", "AST-0000000C", "vehicle", "m3", "s3", "lost"},
		{"n1", "AST-0000000A", "furniture", "m1", "s1", "available"},
		{"n2", "AST-0000000B", "it_equipment", "m2", "s2", "in_use"},
	}
	for _, r := range rows {
		b := baseBody(r.name)
		b["asset_tag"], b["category"], b["model"], b["serial_number"], b["status"] = r.tag, r.cat, r.model, r.serial, r.status
		create(t, e.a, b)
	}
	want := map[string][]string{ // ascending order of names
		"asset_tag":     {"n1", "n2", "n3"},
		"name":          {"n1", "n2", "n3"},
		"category":      {"n1", "n2", "n3"}, // furniture, it_equipment, vehicle
		"model":         {"n1", "n2", "n3"},
		"serial_number": {"n1", "n2", "n3"},
		"status":        {"n1", "n2", "n3"}, // available, in_use, lost
		"created_at":    {"n3", "n1", "n2"},
	}
	for col, asc := range want {
		if got := names(list(t, e.a, "sort="+col+"&dir=asc")); !slices.Equal(got, asc) {
			t.Errorf("%s asc: %v want %v", col, got, asc)
		}
		desc := slices.Clone(asc)
		slices.Reverse(desc)
		if got := names(list(t, e.a, "sort="+col+"&dir=desc")); !slices.Equal(got, desc) {
			t.Errorf("%s desc: %v want %v", col, got, desc)
		}
	}
	// Defaults: created_at desc.
	if got := names(list(t, e.a, "")); !slices.Equal(got, []string{"n2", "n1", "n3"}) {
		t.Errorf("default %v", got)
	}
}

func TestPagination20(t *testing.T) {
	e := setup(t)
	for i := 0; i < 45; i++ {
		create(t, e.a, baseBody(fmt.Sprintf("p%02d", i)))
	}
	seen := map[string]bool{}
	for page, want := range map[int]int{1: 20, 2: 20, 3: 5, 4: 0} {
		l := list(t, e.a, fmt.Sprintf("sort=name&dir=asc&page=%d", page))
		if l.Total != 45 || l.Page != page || l.PageSize != 20 || len(l.Items) != want {
			t.Fatalf("page %d: total %d page %d size %d items %d", page, l.Total, l.Page, l.PageSize, len(l.Items))
		}
		for _, it := range l.Items {
			if seen[it.Name] {
				t.Fatalf("duplicate %s", it.Name)
			}
			seen[it.Name] = true
		}
	}
	if len(seen) != 45 {
		t.Fatalf("seen %d", len(seen))
	}
	// Stable across pages even when the sort key ties.
	ids := map[string]bool{}
	for p := 1; p <= 3; p++ {
		for _, it := range list(t, e.a, fmt.Sprintf("sort=category&page=%d", p)).Items {
			ids[it.ID] = true
		}
	}
	if len(ids) != 45 {
		t.Fatalf("tie-break unstable: %d distinct ids", len(ids))
	}
}

func TestInvalidSort400(t *testing.T) {
	e := setup(t)
	for _, q := range []string{
		"sort=bogus", "sort=name%3Bdrop", "sort=id", "dir=sideways", "page=0", "page=-1", "page=x",
		"category=nope", "status=in_use,nope", "location_id=not-a-uuid", "organization_id=zzz",
	} {
		res := e.a.Do("GET", "/api/assets?"+q, nil)
		if res.StatusCode != 400 {
			t.Errorf("%s: status %d", q, res.StatusCode)
			res.Body.Close()
			continue
		}
		var er errResp
		e.a.JSON(res, &er)
		if er.Error.Code != "invalid_filter" {
			t.Errorf("%s: code %q", q, er.Error.Code)
		}
	}
}

func TestParseListFilterDefaults(t *testing.T) {
	f, err := asset.ParseListFilter(url.Values{"q": {"  hi "}, "status": {"in_use, lost"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Sort != "created_at" || f.Dir != "desc" || f.Page != 1 || strings.TrimSpace(f.Q) != "hi" ||
		!slices.Equal(f.Statuses, []string{"in_use", "lost"}) {
		t.Fatalf("%+v", f)
	}
}

func TestSummaryCounts(t *testing.T) {
	e := setup(t)
	for i, st := range []string{"available", "available", "in_use", "maintenance", "damaged", "lost", "archived", "archived"} {
		b := baseBody(fmt.Sprintf("s%d", i))
		b["status"] = st
		create(t, e.a, b)
	}
	b := baseBody("other tenant")
	create(t, e.b, b)
	res := e.a.Do("GET", "/api/assets/summary?status=lost&q=zzz", nil)
	if res.StatusCode != 200 {
		t.Fatalf("summary %d", res.StatusCode)
	}
	var s map[string]int
	e.a.JSON(res, &s)
	want := map[string]int{"total": 8, "available": 2, "in_use": 1, "needs_attention": 3}
	if len(s) != 4 {
		t.Fatalf("keys %v", s)
	}
	for k, v := range want {
		if s[k] != v {
			t.Errorf("%s = %d want %d (%v)", k, s[k], v, s)
		}
	}
	// Empty tenant returns zeros (not null).
	res = e.b.Do("GET", "/api/assets/summary", nil)
	var sb map[string]int
	e.b.JSON(res, &sb)
	if sb["total"] != 1 {
		t.Fatalf("b %v", sb)
	}
}

func TestListCrossTenantNotVisible(t *testing.T) {
	e := setup(t)
	locB := masterID(t, e.b, "locations", "LocB")
	b := baseBody("secret-b")
	b["location_id"] = locB
	create(t, e.b, b)
	create(t, e.a, baseBody("mine"))
	if l := list(t, e.a, ""); l.Total != 1 || l.Items[0].Name != "mine" {
		t.Fatalf("%v", names(l))
	}
	if l := list(t, e.a, "q=secret"); l.Total != 0 {
		t.Fatalf("search leaked")
	}
	// Foreign location id as a filter yields nothing, not B's rows.
	if l := list(t, e.a, "location_id="+locB); l.Total != 0 {
		t.Fatalf("filter leaked")
	}
	res := e.a.Do("GET", "/api/assets/summary", nil)
	var s map[string]int
	e.a.JSON(res, &s)
	if s["total"] != 1 {
		t.Fatalf("summary leaked %v", s)
	}
}
