// Package masterdata implements locations, organizations and suppliers: three
// tenant-scoped name-keyed lists served by one generic service.
package masterdata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/db"
	"assetku/api/internal/db/gen"
)

// Kind selects the master data table. Only the constants below are valid; the
// table name is interpolated into SQL, so never build a Kind from user input
// without ParseKind.
type Kind string

const (
	Locations     Kind = "locations"
	Organizations Kind = "organizations"
	Suppliers     Kind = "suppliers"
)

// kindFields lists the whitelisted text fields (besides name) per kind.
var kindFields = map[Kind][]string{
	Locations:     {"description"},
	Organizations: {},
	Suppliers:     {"contact_name", "phone", "email", "address"},
}

// ParseKind validates a path segment.
func ParseKind(s string) (Kind, bool) {
	k := Kind(s)
	_, ok := kindFields[k]
	return k, ok
}

// Fields returns the whitelisted extra text columns of the kind.
func (k Kind) Fields() []string { return kindFields[k] }

var (
	ErrNotFound     = errors.New("masterdata: not found")
	ErrNameTaken    = errors.New("masterdata: name taken")
	ErrNameRequired = errors.New("masterdata: name required")
)

// Service serves master data. The unexported pool is used by the HTTP handlers;
// FindByName/FindOrCreateByName accept any gen.DBTX (pgx.Tx, *pgxpool.Pool,
// *pgxpool.Conn) so the importer can call them inside its own transaction.
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service {
	return &Service{pool: pool, auth: a}
}

// Item is a master data row: id, name, the kind's extra fields, asset_count.
type Item map[string]any

func (k Kind) columns() []string { return append([]string{"name"}, kindFields[k]...) }

func (k Kind) selectList() string {
	return "id, " + strings.Join(k.columns(), ", ")
}

func scanItem(row pgx.Row, k Kind) (Item, error) {
	cols := k.columns()
	var id uuid.UUID
	vals := make([]string, len(cols))
	dest := []any{&id}
	for i := range vals {
		dest = append(dest, &vals[i])
	}
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	it := Item{"id": id, "asset_count": int64(0)}
	for i, c := range cols {
		it[c] = vals[i]
	}
	return it, nil
}

// AssetCounts returns how many assets of the tenant reference each row of the
// kind (missing id means 0).
//
// TODO(Task 5): the assets table does not exist yet, so this returns an empty
// map. Replace with a GROUP BY count on assets.location_id / organization_id /
// supplier_id. It backs both the list asset_count and the in_use delete count.
func (s *Service) AssetCounts(ctx context.Context, tenantID uuid.UUID, k Kind) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

// List returns the tenant's rows ordered by name, optionally filtered by a
// case-insensitive substring of the name.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, k Kind, q string) ([]Item, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"
	if q = strings.TrimSpace(q); q != "" {
		esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
		args = append(args, "%"+esc+"%")
		where += " AND name ILIKE $2"
	}
	rows, err := s.pool.Query(ctx, fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s ORDER BY lower(name), id", k.selectList(), k, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows, k)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	counts, err := s.AssetCounts(ctx, tenantID, k)
	if err != nil {
		return nil, err
	}
	for _, it := range out {
		it["asset_count"] = counts[it["id"].(uuid.UUID)]
	}
	return out, nil
}

func mapWriteErr(err error) error {
	if _, ok := db.IsUniqueViolation(err); ok {
		return ErrNameTaken
	}
	return err
}

// Create inserts a row. values holds whitelisted fields only (name required).
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, k Kind, values map[string]string) (Item, error) {
	name := strings.TrimSpace(values["name"])
	if name == "" {
		return nil, ErrNameRequired
	}
	cols, args, ph := []string{"tenant_id", "name"}, []any{tenantID, name}, []string{"$1", "$2"}
	for _, f := range kindFields[k] {
		cols = append(cols, f)
		args = append(args, strings.TrimSpace(values[f]))
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	row := s.pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		k, strings.Join(cols, ", "), strings.Join(ph, ", "), k.selectList()), args...)
	it, err := scanItem(row, k)
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return it, nil
}

// Update changes only the fields present in values (whitelisted keys).
func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, k Kind, values map[string]string) (Item, error) {
	sets, args := []string{"updated_at = now()"}, []any{tenantID, id}
	for _, c := range k.columns() {
		v, ok := values[c]
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if c == "name" && v == "" {
			return nil, ErrNameRequired
		}
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", c, len(args)))
	}
	row := s.pool.QueryRow(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE tenant_id = $1 AND id = $2 RETURNING %s",
		k, strings.Join(sets, ", "), k.selectList()), args...)
	it, err := scanItem(row, k)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapWriteErr(err)
	}
	counts, err := s.AssetCounts(ctx, tenantID, k)
	if err != nil {
		return nil, err
	}
	it["asset_count"] = counts[id]
	return it, nil
}

// Delete removes a row. A foreign-key violation (assets still reference it) is
// returned unchanged so the caller can map it with db.IsFKViolation.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID, k Kind) error {
	tag, err := s.pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND id = $2", k), tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByName looks a row up by trimmed, case-insensitive name. q may be a
// pgx.Tx or the pool.
func (s *Service) FindByName(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, k Kind, name string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, fmt.Sprintf(
		"SELECT id FROM %s WHERE tenant_id = $1 AND lower(btrim(name)) = lower(btrim($2))", k),
		tenantID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// FindOrCreateByName returns the id of the row with that name (trimmed,
// case-insensitive), inserting it when missing. A blank name is ErrNameRequired.
// q may be a pgx.Tx or the pool; the insert is race-safe via ON CONFLICT.
func (s *Service) FindOrCreateByName(ctx context.Context, q gen.DBTX, tenantID uuid.UUID, k Kind, name string) (uuid.UUID, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, false, ErrNameRequired
	}
	var id uuid.UUID
	err := q.QueryRow(ctx, fmt.Sprintf(
		"INSERT INTO %s (tenant_id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING RETURNING id", k),
		tenantID, name).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, err
	}
	id, ok, err := s.FindByName(ctx, q, tenantID, k, name)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !ok {
		return uuid.Nil, false, errors.New("masterdata: row vanished after conflict")
	}
	return id, false, nil
}
