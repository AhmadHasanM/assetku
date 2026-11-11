// Package movement records asset location moves. Adding a movement changes
// the asset's current location; deleting one never does.
package movement

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

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
	"assetku/api/internal/history"
	"assetku/api/internal/masterdata"
)

var ErrNotFound = errors.New("movement: not found")

// ValidationError maps JSON field names to error codes.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("movement: validation failed: %v", e.Fields)
}

// Input is the POST body.
type Input struct {
	ToLocationID *uuid.UUID  `json:"to_location_id"`
	Date         *civil.Date `json:"date"`
	MovedBy      string      `json:"moved_by"`
	Reason       string      `json:"reason"`
}

// Movement is a stored move; from_* is a snapshot, to_location_name is live.
type Movement struct {
	ID               uuid.UUID  `json:"id"`
	FromLocationID   *uuid.UUID `json:"from_location_id"`
	FromLocationName string     `json:"from_location_name"`
	ToLocationID     uuid.UUID  `json:"to_location_id"`
	ToLocationName   string     `json:"to_location_name"`
	Date             civil.Date `json:"date"`
	MovedBy          string     `json:"moved_by"`
	Reason           string     `json:"reason"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service { return &Service{pool: pool, auth: a} }

// List returns the asset's movements, newest date first.
func (s *Service) List(ctx context.Context, tenantID, assetID uuid.UUID) ([]Movement, error) {
	if _, err := asset.Load(ctx, s.pool, tenantID, assetID); errors.Is(err, asset.ErrNotFound) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := gen.New(s.pool).ListMovements(ctx, gen.ListMovementsParams{TenantID: tenantID, AssetID: assetID})
	if err != nil {
		return nil, err
	}
	out := make([]Movement, 0, len(rows))
	for _, r := range rows {
		m := Movement{ID: r.ID, FromLocationName: r.FromLocationName, ToLocationID: r.ToLocationID,
			ToLocationName: r.ToLocationName, Date: civil.DateOf(r.Date.Time), MovedBy: r.MovedBy,
			Reason: r.Reason, CreatedAt: r.CreatedAt}
		if r.FromLocationID.Valid {
			id := r.FromLocationID.UUID
			m.FromLocationID = &id
		}
		out = append(out, m)
	}
	return out, nil
}

// Add moves the asset to in.ToLocationID in one transaction: it locks the
// asset row, snapshots the current location as "from", inserts the movement,
// sets assets.location_id (bumping updated_at) and records movement_added.
func (s *Service) Add(ctx context.Context, p auth.Principal, assetID uuid.UUID, in Input) (Movement, error) {
	var out Movement
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		if _, err := q.LockAsset(ctx, gen.LockAssetParams{TenantID: p.TenantID, ID: assetID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		fields := map[string]string{}
		switch {
		case in.Date == nil:
			fields["date"] = "required"
		case !in.Date.IsValid():
			fields["date"] = "invalid"
		}
		if in.ToLocationID == nil {
			fields["to_location_id"] = "required"
		}
		cur, err := asset.Load(ctx, tx, p.TenantID, assetID)
		if err != nil {
			return err
		}
		var toName string
		if in.ToLocationID != nil {
			switch {
			case cur.LocationID != nil && *cur.LocationID == *in.ToLocationID:
				fields["to_location_id"] = "same_as_current"
			default:
				ok, err := asset.RefBelongsToTenant(ctx, tx, p.TenantID, masterdata.Locations, *in.ToLocationID)
				if err != nil {
					return err
				}
				if !ok {
					fields["to_location_id"] = "invalid"
				}
			}
		}
		if len(fields) > 0 {
			return &ValidationError{Fields: fields}
		}
		movedBy := strings.TrimSpace(in.MovedBy)
		if movedBy == "" {
			movedBy = p.Name
		}
		fromName := ""
		params := gen.InsertMovementParams{
			TenantID: p.TenantID, AssetID: assetID, ToLocationID: *in.ToLocationID,
			Date: pgtype.Date{Time: in.Date.In(time.UTC), Valid: true}, MovedBy: movedBy, Reason: in.Reason,
		}
		if cur.Location != nil {
			fromName = cur.Location.Name
			params.FromLocationID = uuid.NullUUID{UUID: cur.Location.ID, Valid: true}
			params.FromLocationName = fromName
		}
		r, err := q.InsertMovement(ctx, params)
		if err != nil {
			return err
		}
		if _, err := q.SetAssetLocation(ctx, gen.SetAssetLocationParams{
			TenantID: p.TenantID, ID: assetID, LocationID: uuid.NullUUID{UUID: *in.ToLocationID, Valid: true},
		}); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT name FROM locations WHERE tenant_id = $1 AND id = $2`,
			p.TenantID, *in.ToLocationID).Scan(&toName); err != nil {
			return err
		}
		out = Movement{ID: r.ID, FromLocationName: fromName, ToLocationID: *in.ToLocationID,
			ToLocationName: toName, Date: *in.Date, MovedBy: movedBy, Reason: in.Reason, CreatedAt: r.CreatedAt}
		if params.FromLocationID.Valid {
			id := params.FromLocationID.UUID
			out.FromLocationID = &id
		}
		return history.Record(ctx, tx, history.Entry{
			TenantID: p.TenantID, AssetID: assetID, Event: history.EventMovementAdded,
			ActorID: actorID(p), ActorName: p.Name,
			Detail: map[string]any{"from": fromName, "to": toName, "reason": in.Reason},
		})
	})
	return out, err
}

// Delete removes the movement record only; the asset's location is untouched.
func (s *Service) Delete(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		m, err := q.GetMovementForDelete(ctx, gen.GetMovementForDeleteParams{TenantID: p.TenantID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if n, err := q.DeleteMovement(ctx, gen.DeleteMovementParams{TenantID: p.TenantID, ID: id}); err != nil {
			return err
		} else if n == 0 {
			return ErrNotFound
		}
		return history.Record(ctx, tx, history.Entry{
			TenantID: p.TenantID, AssetID: m.AssetID, Event: history.EventMovementDeleted,
			ActorID: actorID(p), ActorName: p.Name,
			Detail: map[string]any{"from": m.FromLocationName, "to": m.ToLocationName, "date": civil.DateOf(m.Date.Time).String()},
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
