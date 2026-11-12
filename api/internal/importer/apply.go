package importer

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/history"
	"assetku/api/internal/masterdata"
)

// Skip reasons recorded in Result.SkippedRows. invalid_value is suffixed with
// ":<field>". ReasonError marks a row whose database write failed (for
// example a tag taken concurrently by another import).
const (
	ReasonMissingName  = "missing_name"
	ReasonDuplicateTag = "duplicate_tag_in_file"
	ReasonInvalidValue = "invalid_value:"
	ReasonError        = "error"
)

// NameCount is a name and the number of rows that carried it.
type NameCount struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

// ValueIssue is an unrecognized status or category value and its row count.
type ValueIssue struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Rows  int    `json:"rows"`
}

// RowIssue is a skipped row: its 1-based spreadsheet row number and reason.
type RowIssue struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// Result summarizes an import. Names and values are grouped
// case-insensitively; the first spelling seen is reported.
type Result struct {
	Created                int          `json:"created"`
	Updated                int          `json:"updated"`
	Unchanged              int          `json:"unchanged"`
	Skipped                int          `json:"skipped"`
	UnknownColumns         []string     `json:"unknown_columns"`
	UnmatchedOrganizations []NameCount  `json:"unmatched_organizations"`
	UnrecognizedValues     []ValueIssue `json:"unrecognized_values"`
	SkippedRows            []RowIssue   `json:"skipped_rows"`
	CreatedSuppliers       []string     `json:"created_suppliers"`
	CreatedLocations       []string     `json:"created_locations"`

	orgIdx   map[string]int
	valueIdx map[string]int
}

// NewResult returns a Result whose lists marshal as [] rather than null.
func NewResult() *Result {
	r := &Result{}
	r.fillEmpty()
	return r
}

func (r *Result) fillEmpty() {
	if r.UnknownColumns == nil {
		r.UnknownColumns = []string{}
	}
	if r.UnmatchedOrganizations == nil {
		r.UnmatchedOrganizations = []NameCount{}
	}
	if r.UnrecognizedValues == nil {
		r.UnrecognizedValues = []ValueIssue{}
	}
	if r.SkippedRows == nil {
		r.SkippedRows = []RowIssue{}
	}
	if r.CreatedSuppliers == nil {
		r.CreatedSuppliers = []string{}
	}
	if r.CreatedLocations == nil {
		r.CreatedLocations = []string{}
	}
}

func (r *Result) skip(row int, reason string) {
	r.Skipped++
	r.SkippedRows = append(r.SkippedRows, RowIssue{Row: row, Reason: reason})
}

func (r *Result) addUnmatchedOrg(name string) {
	if r.orgIdx == nil {
		r.orgIdx = map[string]int{}
		for i, nc := range r.UnmatchedOrganizations {
			r.orgIdx[strings.ToLower(nc.Name)] = i
		}
	}
	key := strings.ToLower(name)
	if i, ok := r.orgIdx[key]; ok {
		r.UnmatchedOrganizations[i].Rows++
		return
	}
	r.orgIdx[key] = len(r.UnmatchedOrganizations)
	r.UnmatchedOrganizations = append(r.UnmatchedOrganizations, NameCount{Name: name, Rows: 1})
}

func (r *Result) addUnrecognized(field, value string) {
	if r.valueIdx == nil {
		r.valueIdx = map[string]int{}
		for i, v := range r.UnrecognizedValues {
			r.valueIdx[v.Field+"\x00"+strings.ToLower(v.Value)] = i
		}
	}
	key := field + "\x00" + strings.ToLower(value)
	if i, ok := r.valueIdx[key]; ok {
		r.UnrecognizedValues[i].Rows++
		return
	}
	r.valueIdx[key] = len(r.UnrecognizedValues)
	r.UnrecognizedValues = append(r.UnrecognizedValues, ValueIssue{Field: field, Value: value, Rows: 1})
}

// The asset and master data services are only used through their
// transaction-scoped methods (CreateTx, UpdateTx, FindByName,
// FindOrCreateByName), which never touch the service's own pool.
var (
	assetCore  = asset.NewService(nil, nil)
	masterCore = masterdata.NewService(nil, nil)
)

// rowNotes collects what a row adds to the Result. It is merged only after
// the row's savepoint is released, so a failed row leaves no trace except
// its skip entry.
type rowNotes struct {
	unmatchedOrg string
	unrecognized [][2]string // field, value
	newSupplier  string
	newLocation  string
}

// ApplyRow imports one data row (rowNum is its 1-based spreadsheet row) into
// tx following spec section 10, inside its own savepoint, and records the
// outcome in res. cols maps field keys to column indexes; seenTags maps
// lowercased tags already seen in this file to their row number.
//
// Row-level problems (missing name, duplicate tag, unparsable values, a
// failed database write) are recorded as skipped rows and return nil. A
// non-nil error means tx itself is unusable (savepoint could not be created
// or rolled back, or ctx ended) and the import must stop.
func ApplyRow(ctx context.Context, tx pgx.Tx, p auth.Principal, cols map[string]int, row []string, rowNum int, res *Result, seenTags map[string]int) error {
	if blankRow(row) {
		return nil
	}
	cell := func(field string) string {
		i, ok := cols[field]
		if !ok || i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	if cell("name") == "" {
		res.skip(rowNum, ReasonMissingName)
		return nil
	}
	tag := cell("asset_tag")
	if tag != "" {
		key := strings.ToLower(tag)
		if _, dup := seenTags[key]; dup {
			res.skip(rowNum, ReasonDuplicateTag)
			return nil
		}
		seenTags[key] = rowNum
	}
	date, err := ParseDate(cell("purchase_date"))
	if err != nil {
		res.skip(rowNum, ReasonInvalidValue+"purchase_date")
		return nil
	}
	cost, err := ParseMoney(cell("purchase_cost"))
	if err != nil {
		res.skip(rowNum, ReasonInvalidValue+"purchase_cost")
		return nil
	}
	warranty, err := ParseWarrantyMonths(cell("warranty_months"))
	if err != nil {
		res.skip(rowNum, ReasonInvalidValue+"warranty_months")
		return nil
	}

	sp, err := tx.Begin(ctx) // savepoint
	if err != nil {
		return err
	}
	outcome, notes, err := applyInSavepoint(ctx, sp, p, cols, cell, tag, date, cost, warranty)
	if err == nil {
		err = sp.Commit(ctx) // release savepoint
	}
	if err != nil {
		if rbErr := sp.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return rbErr
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		res.skip(rowNum, rowErrorReason(err, rowNum))
		return nil
	}
	switch outcome {
	case outcomeCreated:
		res.Created++
	case outcomeUpdated:
		res.Updated++
	default:
		res.Unchanged++
	}
	if notes.unmatchedOrg != "" {
		res.addUnmatchedOrg(notes.unmatchedOrg)
	}
	for _, u := range notes.unrecognized {
		res.addUnrecognized(u[0], u[1])
	}
	if notes.newSupplier != "" {
		res.CreatedSuppliers = append(res.CreatedSuppliers, notes.newSupplier)
	}
	if notes.newLocation != "" {
		res.CreatedLocations = append(res.CreatedLocations, notes.newLocation)
	}
	return nil
}

// rowErrorReason maps a failed write to a skip reason. Asset validation
// failures (e.g. cost >= 1e14, warranty > 1200 months) become
// invalid_value:<field>; anything else, including a tag taken concurrently,
// is "error".
func rowErrorReason(err error, rowNum int) string {
	var ve *asset.ValidationError
	if errors.As(err, &ve) {
		keys := make([]string, 0, len(ve.Fields))
		for k, code := range ve.Fields {
			if code == "invalid" || code == "required" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			sort.Strings(keys)
			return ReasonInvalidValue + keys[0]
		}
		return ReasonError
	}
	log.Printf("import: row %d: %v", rowNum, err)
	return ReasonError
}

func blankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

type outcome int

const (
	outcomeUnchanged outcome = iota
	outcomeCreated
	outcomeUpdated
)

func applyInSavepoint(ctx context.Context, tx pgx.Tx, p auth.Principal, cols map[string]int, cell func(string) string, tag string,
	date *civil.Date, cost *decimal.Decimal, warranty *int) (outcome, rowNotes, error) {
	var notes rowNotes
	var existing *asset.Asset
	if tag != "" {
		a, found, err := asset.FindByTag(ctx, tx, p.TenantID, tag)
		if err != nil {
			return 0, notes, err
		}
		if found {
			existing = &a
		}
	}

	var in asset.Input
	if existing != nil {
		in = existing.Input // only non-empty cells overwrite; the tag stays as stored
	} else {
		in.AssetTag = tag
	}
	setText := func(dst *string, field string) {
		if v := cell(field); v != "" {
			*dst = v
		}
	}
	setText(&in.Name, "name")
	setText(&in.Manufacturer, "manufacturer")
	setText(&in.Model, "model")
	setText(&in.SerialNumber, "serial_number")
	setText(&in.AssignedTo, "assigned_to")
	setText(&in.OrderNumber, "order_number")
	setText(&in.Notes, "notes")
	if date != nil {
		in.PurchaseDate = date
	}
	if cost != nil {
		in.PurchaseCost = cost
	}
	if warranty != nil {
		in.WarrantyMonths = warranty
	}

	// Category and status: an unrecognized value is reported; a new asset
	// falls back to other/available, an existing one keeps its value. A blank
	// value leaves an existing asset alone; on a new asset it falls back too and
	// is reported (as "") only when the column exists in the file.
	for _, e := range []struct {
		field string
		dst   *string
		parse func(string) (string, bool)
	}{
		{"category", &in.Category, ParseCategory},
		{"status", &in.Status, ParseStatus},
	} {
		raw := cell(e.field)
		code, ok := e.parse(raw)
		switch {
		case raw == "" && existing != nil:
		case raw == "":
			*e.dst = code
			if _, mapped := cols[e.field]; mapped {
				notes.unrecognized = append(notes.unrecognized, [2]string{e.field, ""})
			}
		case ok:
			*e.dst = code
		default:
			notes.unrecognized = append(notes.unrecognized, [2]string{e.field, raw})
			if existing == nil {
				*e.dst = code
			}
		}
	}

	if name := cell("organization"); name != "" {
		id, found, err := masterCore.FindByName(ctx, tx, p.TenantID, masterdata.Organizations, name)
		if err != nil {
			return 0, notes, err
		}
		if found {
			in.OrganizationID = &id
		} else {
			notes.unmatchedOrg = name // a new asset gets none; an existing one keeps its own
		}
	}
	for _, m := range []struct {
		field   string
		kind    masterdata.Kind
		dst     **uuid.UUID
		created *string
	}{
		{"supplier", masterdata.Suppliers, &in.SupplierID, &notes.newSupplier},
		{"location", masterdata.Locations, &in.LocationID, &notes.newLocation},
	} {
		name := cell(m.field)
		if name == "" {
			continue
		}
		id, created, err := masterCore.FindOrCreateByName(ctx, tx, p.TenantID, m.kind, name)
		if err != nil {
			return 0, notes, err
		}
		*m.dst = &id
		if created {
			*m.created = name
		}
	}

	if existing == nil {
		if _, err := assetCore.CreateTx(ctx, tx, p, in, history.EventAssetImportedCreated); err != nil {
			return 0, notes, err
		}
		return outcomeCreated, notes, nil
	}
	_, changed, err := assetCore.UpdateTx(ctx, tx, p, existing.ID, in, history.EventAssetImportedUpdated)
	if err != nil {
		return 0, notes, err
	}
	if len(changed) == 0 {
		return outcomeUnchanged, notes, nil
	}
	return outcomeUpdated, notes, nil
}
