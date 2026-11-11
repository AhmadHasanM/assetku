package asset

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"assetku/api/internal/httpx"
)

// PageSize is the number of rows per list page.
const PageSize = 20

// sortColumns maps the allowed sort parameter values to SQL expressions.
// Sort input is only ever looked up here, never interpolated.
var sortColumns = map[string]string{
	"asset_tag":     "lower(a.asset_tag)",
	"name":          "lower(a.name)",
	"category":      "a.category",
	"model":         "lower(a.model)",
	"serial_number": "lower(a.serial_number)",
	"status":        "a.status",
	"created_at":    "a.created_at",
}

// ListFilter holds the parsed list/search/sort/page parameters. Export reuses it.
type ListFilter struct {
	Q                          string
	Categories, Statuses       []string
	LocationID, OrganizationID *uuid.UUID
	Sort, Dir                  string
	Page                       int
}

func errInvalidFilter(param string) error {
	return httpx.Error{Status: http.StatusBadRequest, Code: "invalid_filter", Message: "invalid filter: " + param}
}

// ParseListFilter parses and validates list query parameters. Invalid values
// return a 400 invalid_filter httpx.Error.
func ParseListFilter(v url.Values) (ListFilter, error) {
	f := ListFilter{Q: strings.TrimSpace(v.Get("q")), Sort: "created_at", Dir: "desc", Page: 1}
	var err error
	if f.Categories, err = parseEnums(v.Get("category"), ValidCategory); err != nil {
		return f, errInvalidFilter("category")
	}
	if f.Statuses, err = parseEnums(v.Get("status"), ValidStatus); err != nil {
		return f, errInvalidFilter("status")
	}
	if f.LocationID, err = parseUUID(v.Get("location_id")); err != nil {
		return f, errInvalidFilter("location_id")
	}
	if f.OrganizationID, err = parseUUID(v.Get("organization_id")); err != nil {
		return f, errInvalidFilter("organization_id")
	}
	if s := v.Get("sort"); s != "" {
		if _, ok := sortColumns[s]; !ok {
			return f, errInvalidFilter("sort")
		}
		f.Sort = s
	}
	if d := v.Get("dir"); d != "" {
		if d != "asc" && d != "desc" {
			return f, errInvalidFilter("dir")
		}
		f.Dir = d
	}
	if p := v.Get("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return f, errInvalidFilter("page")
		}
		f.Page = n
	}
	return f, nil
}

// parseEnums splits a comma list, trimming blanks; any unknown value is an error.
func parseEnums(s string, valid func(string) bool) ([]string, error) {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if !valid(p) {
			return nil, errInvalidFilter("enum")
		}
		out = append(out, p)
	}
	return out, nil
}

func parseUUID(s string) (*uuid.UUID, error) {
	if s = strings.TrimSpace(s); s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
