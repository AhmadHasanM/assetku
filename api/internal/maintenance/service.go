// Package maintenance implements per-asset maintenance records (add, list,
// delete; no edit) and the "next due" rule shared with the reminder job.
package maintenance

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

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/history"
)

var ErrNotFound = errors.New("maintenance: not found")

// ValidationError maps JSON field names to error codes.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("maintenance: validation failed: %v", e.Fields)
}

const (
	// DefaultReminderDays applies when next_date is set and reminder_days is null.
	DefaultReminderDays = 7
	maxReminderDays     = 3650
)

// Types are the allowed maintenance types.
var Types = []string{"preventive", "corrective", "inspection", "other"}

// maxCost is the exclusive upper bound of numeric(16,2).
var maxCost = decimal.New(1, 14)

// Input is the POST body.
type Input struct {
	Date         *civil.Date      `json:"date"`
	Type         string           `json:"type"`
	PerformedBy  string           `json:"performed_by"`
	Cost         *decimal.Decimal `json:"cost"`
	NextDate     *civil.Date      `json:"next_date"`
	ReminderDays *int             `json:"reminder_days"`
	Notes        string           `json:"notes"`
}

// Record is a stored maintenance record. Cost is rendered as a fixed
// two-decimal JSON string.
type Record struct {
	ID            uuid.UUID   `json:"id"`
	Date          civil.Date  `json:"date"`
	Type          string      `json:"type"`
	PerformedBy   string      `json:"performed_by"`
	Cost          *string     `json:"cost"`
	NextDate      *civil.Date `json:"next_date"`
	ReminderDays  *int        `json:"reminder_days"`
	Notes         string      `json:"notes"`
	CreatedByName string      `json:"created_by_name"`
	CreatedAt     time.Time   `json:"created_at"`
}

// NextDue points at the latest record when it carries a next_date.
type NextDue struct {
	Date     civil.Date `json:"date"`
	RecordID uuid.UUID  `json:"record_id"`
}

// List is the GET response.
type List struct {
	Items   []Record `json:"items"`
	NextDue *NextDue `json:"next_due"`
}

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service { return &Service{pool: pool, auth: a} }

func validate(in *Input) (Input, map[string]string) {
	in.Type = strings.TrimSpace(in.Type)
	in.PerformedBy = strings.TrimSpace(in.PerformedBy)
	fields := map[string]string{}
	switch {
	case in.Date == nil:
		fields["date"] = "required"
	case !in.Date.IsValid():
		fields["date"] = "invalid"
	}
	switch {
	case in.Type == "":
		fields["type"] = "required"
	case !contains(Types, in.Type):
		fields["type"] = "invalid"
	}
	if in.Cost != nil {
		c := in.Cost.Round(2)
		in.Cost = &c
		if c.IsNegative() || c.GreaterThanOrEqual(maxCost) {
			fields["cost"] = "invalid"
		}
	}
	if in.NextDate != nil {
		switch {
		case !in.NextDate.IsValid():
			fields["next_date"] = "invalid"
		case fields["date"] == "" && in.NextDate.Before(*in.Date):
			fields["next_date"] = "before_date"
		}
	}
	if in.ReminderDays != nil && (*in.ReminderDays < 0 || *in.ReminderDays > maxReminderDays) {
		fields["reminder_days"] = "invalid"
	}
	if in.NextDate != nil && in.ReminderDays == nil {
		d := DefaultReminderDays
		in.ReminderDays = &d
	}
	return *in, fields
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func toDate(d civil.Date) pgtype.Date { return pgtype.Date{Time: d.In(time.UTC), Valid: true} }

func fromDate(d pgtype.Date) *civil.Date {
	if !d.Valid {
		return nil
	}
	c := civil.DateOf(d.Time)
	return &c
}

type row struct {
	ID            uuid.UUID
	Date          pgtype.Date
	Type          string
	PerformedBy   string
	Cost          pgtype.Numeric
	NextDate      pgtype.Date
	ReminderDays  pgtype.Int4
	Notes         string
	CreatedByName string
	CreatedAt     time.Time
}

func (r row) record() Record {
	rec := Record{ID: r.ID, Date: *fromDate(r.Date), Type: r.Type, PerformedBy: r.PerformedBy,
		NextDate: fromDate(r.NextDate), Notes: r.Notes, CreatedByName: r.CreatedByName, CreatedAt: r.CreatedAt}
	if r.Cost.Valid {
		s := decimal.NewFromBigInt(r.Cost.Int, r.Cost.Exp).StringFixed(2)
		rec.Cost = &s
	}
	if r.ReminderDays.Valid {
		n := int(r.ReminderDays.Int32)
		rec.ReminderDays = &n
	}
	return rec
}

// NextDueFor returns the asset's next due maintenance: the LATEST record
// (ORDER BY date DESC, created_at DESC, id DESC LIMIT 1) if and only if it
// has a next_date. Older records never count once a newer one exists. The
// reminder job (Task 8) must apply the same rule; this wraps the
// LatestMaintenanceWithNextDate query. q may be a pgx.Tx or the pool.
func NextDueFor(ctx context.Context, q gen.DBTX, tenantID, assetID uuid.UUID) (*NextDue, error) {
	r, err := gen.New(q).LatestMaintenanceWithNextDate(ctx, gen.LatestMaintenanceWithNextDateParams{TenantID: tenantID, AssetID: assetID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &NextDue{Date: *fromDate(r.NextDate), RecordID: r.ID}, nil
}

// List returns the asset's records (newest first) and its next_due.
func (s *Service) List(ctx context.Context, tenantID, assetID uuid.UUID) (List, error) {
	if _, err := asset.Load(ctx, s.pool, tenantID, assetID); errors.Is(err, asset.ErrNotFound) {
		return List{}, ErrNotFound
	} else if err != nil {
		return List{}, err
	}
	rows, err := gen.New(s.pool).ListMaintenance(ctx, gen.ListMaintenanceParams{TenantID: tenantID, AssetID: assetID})
	if err != nil {
		return List{}, err
	}
	out := List{Items: make([]Record, 0, len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, row(r).record())
	}
	out.NextDue, err = NextDueFor(ctx, s.pool, tenantID, assetID)
	return out, err
}

// Add validates and stores a record and writes maintenance_added, atomically.
func (s *Service) Add(ctx context.Context, p auth.Principal, assetID uuid.UUID, in Input) (Record, error) {
	var rec Record
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		if _, err := q.LockAsset(ctx, gen.LockAssetParams{TenantID: p.TenantID, ID: assetID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		v, fields := validate(&in)
		if len(fields) > 0 {
			return &ValidationError{Fields: fields}
		}
		params := gen.InsertMaintenanceParams{
			TenantID: p.TenantID, AssetID: assetID, Date: toDate(*v.Date), Type: v.Type,
			PerformedBy: v.PerformedBy, Notes: v.Notes, CreatedByName: p.Name,
		}
		if v.Cost != nil {
			params.Cost = pgtype.Numeric{Int: v.Cost.Coefficient(), Exp: v.Cost.Exponent(), Valid: true}
		}
		if v.NextDate != nil {
			params.NextDate = toDate(*v.NextDate)
		}
		if v.ReminderDays != nil {
			params.ReminderDays = pgtype.Int4{Int32: int32(*v.ReminderDays), Valid: true}
		}
		r, err := q.InsertMaintenance(ctx, params)
		if err != nil {
			return err
		}
		rec = row(r).record()
		return history.Record(ctx, tx, history.Entry{
			TenantID: p.TenantID, AssetID: assetID, Event: history.EventMaintenanceAdded,
			ActorID: actorID(p), ActorName: p.Name,
			Detail: map[string]any{"record_id": rec.ID, "type": rec.Type, "date": rec.Date.String()},
		})
	})
	return rec, err
}

// Delete removes the record (tenant-scoped) and writes maintenance_deleted.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		r, err := q.LockMaintenance(ctx, gen.LockMaintenanceParams{TenantID: p.TenantID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if n, err := q.DeleteMaintenance(ctx, gen.DeleteMaintenanceParams{TenantID: p.TenantID, ID: id}); err != nil {
			return err
		} else if n == 0 {
			return ErrNotFound
		}
		return history.Record(ctx, tx, history.Entry{
			TenantID: p.TenantID, AssetID: r.AssetID, Event: history.EventMaintenanceDeleted,
			ActorID: actorID(p), ActorName: p.Name,
			Detail: map[string]any{"type": r.Type, "date": fromDate(r.Date).String()},
		})
	})
}

func actorID(p auth.Principal) *uuid.UUID {
	if p.UserID == uuid.Nil {
		return nil
	}
	id := p.UserID
	return &id
}
