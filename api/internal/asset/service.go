package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"assetku/api/internal/auth"
	"assetku/api/internal/db"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/history"
	"assetku/api/internal/masterdata"
)

var ErrNotFound = errors.New("asset: not found")

// ValidationError maps JSON field names to error codes (required, invalid, taken).
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("asset: validation failed: %v", e.Fields)
}

const (
	// tagAttempts bounds retries when a generated tag collides.
	tagAttempts = 5
	// maxWarrantyMonths keeps WarrantyExpires within a sane date range.
	maxWarrantyMonths = 1200
)

// maxCost is the exclusive upper bound of numeric(16,2).
var maxCost = decimal.New(1, 14)

// Service implements asset CRUD. Create/Update/Get/Delete run on the pool;
// CreateTx/UpdateTx run inside a caller's transaction (e.g. the importer).
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service {
	return &Service{pool: pool, auth: a}
}

// normalize trims text fields and rounds the cost to cents, then validates
// enums and ranges. It does not check references (see CheckRefs).
func normalize(in *Input) map[string]string {
	for _, f := range []*string{&in.Name, &in.Category, &in.Manufacturer, &in.Model, &in.SerialNumber,
		&in.Status, &in.AssignedTo, &in.OrderNumber} {
		*f = strings.TrimSpace(*f)
	}
	in.AssetTag = NormalizeTag(in.AssetTag)
	if in.PurchaseCost != nil {
		c := in.PurchaseCost.Round(2)
		in.PurchaseCost = &c
	}
	fields := map[string]string{}
	if in.Name == "" {
		fields["name"] = "required"
	}
	switch {
	case in.Category == "":
		fields["category"] = "required"
	case !ValidCategory(in.Category):
		fields["category"] = "invalid"
	}
	switch {
	case in.Status == "":
		fields["status"] = "required"
	case !ValidStatus(in.Status):
		fields["status"] = "invalid"
	}
	if c := in.PurchaseCost; c != nil && (c.IsNegative() || c.GreaterThanOrEqual(maxCost)) {
		fields["purchase_cost"] = "invalid"
	}
	if m := in.WarrantyMonths; m != nil && (*m < 0 || *m > maxWarrantyMonths) {
		fields["warranty_months"] = "invalid"
	}
	if d := in.PurchaseDate; d != nil && !d.IsValid() {
		fields["purchase_date"] = "invalid"
	}
	return fields
}

// RefBelongsToTenant reports whether the master data row id of kind k exists
// in the tenant. q may be a pgx.Tx or the pool.
func RefBelongsToTenant(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, k masterdata.Kind, id uuid.UUID) (bool, error) {
	if _, ok := masterdata.ParseKind(string(k)); !ok {
		return false, fmt.Errorf("asset: unknown master data kind %q", k)
	}
	var ok bool
	err := q.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE tenant_id = $1 AND id = $2)", k), tenantID, id).Scan(&ok)
	return ok, err
}

// CheckRefs returns field errors ("invalid") for location_id, organization_id
// and supplier_id that are set but do not belong to the tenant.
func CheckRefs(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, in Input) (map[string]string, error) {
	fields := map[string]string{}
	for _, r := range []struct {
		field string
		kind  masterdata.Kind
		id    *uuid.UUID
	}{
		{"location_id", masterdata.Locations, in.LocationID},
		{"organization_id", masterdata.Organizations, in.OrganizationID},
		{"supplier_id", masterdata.Suppliers, in.SupplierID},
	} {
		if r.id == nil {
			continue
		}
		ok, err := RefBelongsToTenant(ctx, q, tenantID, r.kind, *r.id)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields[r.field] = "invalid"
		}
	}
	return fields, nil
}

// validate normalizes in and returns a *ValidationError when anything is wrong.
func validate(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, in *Input) error {
	fields := normalize(in)
	refFields, err := CheckRefs(ctx, q, tenantID, *in)
	if err != nil {
		return err
	}
	for k, v := range refFields {
		fields[k] = v
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func taken() error { return &ValidationError{Fields: map[string]string{"asset_tag": "taken"}} }

func actorID(p auth.Principal) *uuid.UUID {
	if p.UserID == uuid.Nil {
		return nil
	}
	id := p.UserID
	return &id
}

// Create inserts an asset and records asset_created, in one transaction.
func (s *Service) Create(ctx context.Context, p auth.Principal, in Input) (Asset, error) {
	var a Asset
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		a, err = s.CreateTx(ctx, tx, p, in, history.EventAssetCreated)
		return err
	})
	return a, err
}

// CreateTx validates and inserts in inside tx and records event
// (history.EventAssetCreated or history.EventAssetImportedCreated). A blank tag
// is generated, retrying collisions; a taken user tag is a ValidationError.
// Collisions never abort tx.
func (s *Service) CreateTx(ctx context.Context, tx pgx.Tx, p auth.Principal, in Input, event string) (Asset, error) {
	if err := validate(ctx, tx, p.TenantID, &in); err != nil {
		return Asset{}, err
	}
	q := gen.New(tx)
	auto := in.AssetTag == ""
	var id uuid.UUID
	for attempt := 0; ; attempt++ {
		if auto {
			in.AssetTag = GenerateTag()
		}
		params := insertParams(p.TenantID, in)
		var err error
		id, err = q.InsertAsset(ctx, params)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, err
		}
		if !auto {
			return Asset{}, taken()
		}
		if attempt+1 == tagAttempts {
			return Asset{}, fmt.Errorf("asset: no free tag after %d attempts", tagAttempts)
		}
	}
	err := history.Record(ctx, tx, history.Entry{TenantID: p.TenantID, AssetID: id, Event: event,
		ActorID: actorID(p), ActorName: p.Name})
	if err != nil {
		return Asset{}, err
	}
	return load(ctx, tx, p.TenantID, id)
}

// Update replaces all editable fields and records history, in one transaction.
func (s *Service) Update(ctx context.Context, p auth.Principal, id uuid.UUID, in Input) (Asset, error) {
	var a Asset
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		a, _, err = s.UpdateTx(ctx, tx, p, id, in, "")
		return err
	})
	return a, err
}

// UpdateTx replaces all editable fields of the tenant's asset id inside tx and
// returns the stored asset plus the JSON names of the changed fields (empty:
// nothing written, no history).
//
// With event == "" (form edit) history gets one entry per kind of change:
// status_changed, location_changed (names), assignee_changed {from,to} and one
// details_updated {fields} for everything else. With an explicit event (e.g.
// history.EventAssetImportedUpdated) a single entry {fields: all changed} is
// recorded instead.
func (s *Service) UpdateTx(ctx context.Context, tx pgx.Tx, p auth.Principal, id uuid.UUID, in Input, event string) (Asset, []string, error) {
	q := gen.New(tx)
	if _, err := q.LockAsset(ctx, gen.LockAssetParams{TenantID: p.TenantID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, nil, ErrNotFound
	} else if err != nil {
		return Asset{}, nil, err
	}
	if err := validate(ctx, tx, p.TenantID, &in); err != nil {
		return Asset{}, nil, err
	}
	old, err := load(ctx, tx, p.TenantID, id)
	if err != nil {
		return Asset{}, nil, err
	}
	if in.AssetTag == "" {
		in.AssetTag = old.AssetTag // a blank tag keeps the printed one
	}
	changed := diff(old.Input, in)
	if len(changed) == 0 {
		return old, nil, nil
	}
	if in.AssetTag != old.AssetTag {
		other, err := q.FindAssetIDByTag(ctx, gen.FindAssetIDByTagParams{TenantID: p.TenantID, AssetTag: in.AssetTag})
		if err == nil && other != id {
			return Asset{}, nil, taken()
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Asset{}, nil, err
		}
	}
	ip := insertParams(p.TenantID, in)
	_, err = q.UpdateAsset(ctx, gen.UpdateAssetParams{
		TenantID: p.TenantID, ID: id, Name: ip.Name, AssetTag: ip.AssetTag, Category: ip.Category,
		Manufacturer: ip.Manufacturer, Model: ip.Model, SerialNumber: ip.SerialNumber, Status: ip.Status,
		LocationID: ip.LocationID, AssignedTo: ip.AssignedTo, OrganizationID: ip.OrganizationID,
		SupplierID: ip.SupplierID, OrderNumber: ip.OrderNumber, PurchaseDate: ip.PurchaseDate,
		PurchaseCost: ip.PurchaseCost, WarrantyMonths: ip.WarrantyMonths, Notes: ip.Notes,
	})
	if _, unique := db.IsUniqueViolation(err); unique {
		return Asset{}, nil, taken() // concurrent insert of the same tag
	}
	if err != nil {
		return Asset{}, nil, err
	}
	cur, err := load(ctx, tx, p.TenantID, id)
	if err != nil {
		return Asset{}, nil, err
	}
	for _, e := range historyEntries(old, cur, changed, event) {
		e.TenantID, e.AssetID, e.ActorID, e.ActorName = p.TenantID, id, actorID(p), p.Name
		if err := history.Record(ctx, tx, e); err != nil {
			return Asset{}, nil, err
		}
	}
	return cur, changed, nil
}

func refName(r *Ref) string {
	if r == nil {
		return ""
	}
	return r.Name
}

func historyEntries(old, cur Asset, changed []string, event string) []history.Entry {
	if event != "" {
		return []history.Entry{{Event: event, Detail: map[string]any{"fields": changed}}}
	}
	var out []history.Entry
	var details []string
	for _, f := range changed {
		switch f {
		case "status":
			out = append(out, history.Entry{Event: history.EventStatusChanged,
				Detail: map[string]any{"from": old.Status, "to": cur.Status}})
		case "location_id":
			out = append(out, history.Entry{Event: history.EventLocationChanged,
				Detail: map[string]any{"from": refName(old.Location), "to": refName(cur.Location)}})
		case "assigned_to":
			out = append(out, history.Entry{Event: history.EventAssigneeChanged,
				Detail: map[string]any{"from": old.AssignedTo, "to": cur.AssignedTo}})
		default:
			details = append(details, f)
		}
	}
	if len(details) > 0 {
		out = append(out, history.Entry{Event: history.EventDetailsUpdated, Detail: map[string]any{"fields": details}})
	}
	return out
}

func eqPtr[T comparable](a, b *T) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// diff lists the JSON names of fields that differ, in Input field order.
func diff(a, b Input) []string {
	var out []string
	add := func(name string, same bool) {
		if !same {
			out = append(out, name)
		}
	}
	add("name", a.Name == b.Name)
	add("asset_tag", a.AssetTag == b.AssetTag)
	add("category", a.Category == b.Category)
	add("manufacturer", a.Manufacturer == b.Manufacturer)
	add("model", a.Model == b.Model)
	add("serial_number", a.SerialNumber == b.SerialNumber)
	add("status", a.Status == b.Status)
	add("location_id", eqPtr(a.LocationID, b.LocationID))
	add("assigned_to", a.AssignedTo == b.AssignedTo)
	add("organization_id", eqPtr(a.OrganizationID, b.OrganizationID))
	add("supplier_id", eqPtr(a.SupplierID, b.SupplierID))
	add("order_number", a.OrderNumber == b.OrderNumber)
	add("purchase_date", eqPtr(a.PurchaseDate, b.PurchaseDate))
	add("purchase_cost", (a.PurchaseCost == nil && b.PurchaseCost == nil) ||
		(a.PurchaseCost != nil && b.PurchaseCost != nil && a.PurchaseCost.Equal(*b.PurchaseCost)))
	add("warranty_months", eqPtr(a.WarrantyMonths, b.WarrantyMonths))
	add("notes", a.Notes == b.Notes)
	return out
}

// Get returns the tenant's asset or ErrNotFound.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (Asset, error) {
	return load(ctx, s.pool, tenantID, id)
}

// Load is Get on an arbitrary querier (pgx.Tx or pool).
func Load(ctx context.Context, q gen.DBTX, tenantID, id uuid.UUID) (Asset, error) {
	return load(ctx, q, tenantID, id)
}

// FindByTag returns the tenant's asset whose tag matches case-insensitively.
func FindByTag(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, tag string) (Asset, bool, error) {
	id, err := gen.New(q).FindAssetIDByTag(ctx, gen.FindAssetIDByTagParams{TenantID: tenantID, AssetTag: NormalizeTag(tag)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, false, nil
	}
	if err != nil {
		return Asset{}, false, err
	}
	a, err := load(ctx, q, tenantID, id)
	return a, err == nil, err
}

// Delete hard-deletes the asset; history (and later maintenance/movements) cascade.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	n, err := gen.New(s.pool).DeleteAsset(ctx, gen.DeleteAssetParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AssigneeSuggestions returns up to 10 distinct non-empty assignees of the
// tenant containing q (case-insensitive, literal), most frequent first.
func (s *Service) AssigneeSuggestions(ctx context.Context, tenantID uuid.UUID, q string) ([]string, error) {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q))
	out, err := gen.New(s.pool).AssigneeSuggestions(ctx, gen.AssigneeSuggestionsParams{TenantID: tenantID, Pattern: esc})
	if out == nil {
		out = []string{}
	}
	return out, err
}

func load(ctx context.Context, q gen.DBTX, tenantID, id uuid.UUID) (Asset, error) {
	r, err := gen.New(q).GetAsset(ctx, gen.GetAssetParams{TenantID: tenantID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	a := Asset{
		ID: r.ID,
		Input: Input{
			Name: r.Name, AssetTag: r.AssetTag, Category: r.Category, Manufacturer: r.Manufacturer,
			Model: r.Model, SerialNumber: r.SerialNumber, Status: r.Status, AssignedTo: r.AssignedTo,
			OrderNumber: r.OrderNumber, Notes: r.Notes,
			LocationID: fromNullUUID(r.LocationID), OrganizationID: fromNullUUID(r.OrganizationID),
			SupplierID: fromNullUUID(r.SupplierID),
		},
		Location:     ref(r.LocationID, r.LocationName),
		Organization: ref(r.OrganizationID, r.OrganizationName),
		Supplier:     ref(r.SupplierID, r.SupplierName),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.PurchaseDate.Valid {
		d := civil.DateOf(r.PurchaseDate.Time)
		a.PurchaseDate = &d
	}
	if r.PurchaseCost.Valid {
		c := decimal.NewFromBigInt(r.PurchaseCost.Int, r.PurchaseCost.Exp)
		a.PurchaseCost = &c
	}
	if r.WarrantyMonths.Valid {
		m := int(r.WarrantyMonths.Int32)
		a.WarrantyMonths = &m
	}
	a.WarrantyExpires = WarrantyExpires(a.PurchaseDate, a.WarrantyMonths)
	a.WarrantyExpired = a.WarrantyExpires != nil && a.WarrantyExpires.Before(Today())
	return a, nil
}

func ref(id uuid.NullUUID, name *string) *Ref {
	if !id.Valid || name == nil {
		return nil
	}
	return &Ref{ID: id.UUID, Name: *name}
}

func fromNullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	return &n.UUID
}

func toNullUUID(p *uuid.UUID) uuid.NullUUID {
	if p == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *p, Valid: true}
}

func insertParams(tenantID uuid.UUID, in Input) gen.InsertAssetParams {
	p := gen.InsertAssetParams{
		TenantID: tenantID, Name: in.Name, AssetTag: in.AssetTag, Category: in.Category,
		Manufacturer: in.Manufacturer, Model: in.Model, SerialNumber: in.SerialNumber, Status: in.Status,
		LocationID: toNullUUID(in.LocationID), AssignedTo: in.AssignedTo,
		OrganizationID: toNullUUID(in.OrganizationID), SupplierID: toNullUUID(in.SupplierID),
		OrderNumber: in.OrderNumber, Notes: in.Notes,
	}
	if in.PurchaseDate != nil {
		p.PurchaseDate = pgtype.Date{Time: in.PurchaseDate.In(time.UTC), Valid: true}
	}
	if in.PurchaseCost != nil {
		p.PurchaseCost = pgtype.Numeric{Int: in.PurchaseCost.Coefficient(), Exp: in.PurchaseCost.Exponent(), Valid: true}
	}
	if in.WarrantyMonths != nil {
		p.WarrantyMonths = pgtype.Int4{Int32: int32(*in.WarrantyMonths), Valid: true}
	}
	return p
}
