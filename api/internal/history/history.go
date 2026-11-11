// Package history records and serves the per-asset activity log.
package history

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"assetku/api/internal/db/gen"
)

// Event codes stored in asset_history.event.
const (
	EventAssetCreated         = "asset_created"
	EventAssetImportedCreated = "asset_imported_created"
	EventAssetImportedUpdated = "asset_imported_updated"

	EventStatusChanged   = "status_changed"
	EventLocationChanged = "location_changed"
	EventAssigneeChanged = "assignee_changed"
	EventDetailsUpdated  = "details_updated"

	EventLabelPrinted = "label_printed"
	EventBastPrinted  = "bast_printed"

	EventMaintenanceAdded   = "maintenance_added"
	EventMaintenanceDeleted = "maintenance_deleted"
	EventMovementAdded      = "movement_added"
	EventMovementDeleted    = "movement_deleted"
)

// Entry is one history row. ActorName is a snapshot of the actor's name;
// a nil Detail is stored as {}.
type Entry struct {
	TenantID, AssetID uuid.UUID
	Event             string
	ActorID           *uuid.UUID
	ActorName         string
	Detail            map[string]any
}

// Record inserts e. q may be a pgx.Tx (to write in the caller's transaction)
// or the pool.
func Record(ctx context.Context, q gen.DBTX, e Entry) error {
	detail := e.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	actor := uuid.NullUUID{}
	if e.ActorID != nil {
		actor = uuid.NullUUID{UUID: *e.ActorID, Valid: true}
	}
	return gen.New(q).InsertHistory(ctx, gen.InsertHistoryParams{
		TenantID: e.TenantID, AssetID: e.AssetID, Event: e.Event,
		ActorID: actor, ActorName: e.ActorName, Detail: b,
	})
}
