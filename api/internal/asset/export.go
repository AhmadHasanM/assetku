package asset

import (
	"context"
	"errors"
	"strconv"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// ExportMaxRows is the hard cap on rows in one export; it protects memory.
const ExportMaxRows = 50000

// ErrExportTooLarge is returned by ListForExport when more than ExportMaxRows rows match.
var ErrExportTooLarge = errors.New("asset: export exceeds row limit")

// ExportRow is one asset with every exportable field and names resolved.
// It is separate from ListItem so the list endpoint's JSON stays unchanged.
type ExportRow struct {
	ListItem
	Manufacturer   string
	SupplierName   string
	OrderNumber    string
	PurchaseDate   *civil.Date
	PurchaseCost   *decimal.Decimal
	WarrantyMonths *int
	Notes          string
}

// ListForExport returns all rows matching f (ignoring f.Page) in the list
// order, or ErrExportTooLarge when more than ExportMaxRows match.
func (s *Service) ListForExport(ctx context.Context, tenantID uuid.UUID, f ListFilter) ([]ExportRow, error) {
	where, args := f.where(tenantID)
	args = append(args, ExportMaxRows+1)
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.asset_tag, a.name, a.category, a.model, a.serial_number, a.status, a.assigned_to,
		COALESCE(l.name, ''), COALESCE(o.name, ''),
		a.manufacturer, COALESCE(sp.name, ''), a.order_number, a.purchase_date, a.purchase_cost, a.warranty_months, a.notes
		FROM assets a
		LEFT JOIN locations l ON l.id = a.location_id AND l.tenant_id = a.tenant_id
		LEFT JOIN organizations o ON o.id = a.organization_id AND o.tenant_id = a.tenant_id
		LEFT JOIN suppliers sp ON sp.id = a.supplier_id AND sp.tenant_id = a.tenant_id
		WHERE `+where+" ORDER BY "+f.orderBy()+" LIMIT $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExportRow{}
	for rows.Next() {
		var r ExportRow
		var pd pgtype.Date
		var cost pgtype.Numeric
		if err := rows.Scan(&r.ID, &r.AssetTag, &r.Name, &r.Category, &r.Model, &r.SerialNumber, &r.Status, &r.AssignedTo,
			&r.LocationName, &r.OrganizationName, &r.Manufacturer, &r.SupplierName, &r.OrderNumber,
			&pd, &cost, &r.WarrantyMonths, &r.Notes); err != nil {
			return nil, err
		}
		if pd.Valid {
			d := civil.DateOf(pd.Time)
			r.PurchaseDate = &d
		}
		if cost.Valid && cost.Int != nil {
			c := decimal.NewFromBigInt(cost.Int, cost.Exp)
			r.PurchaseCost = &c
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) > ExportMaxRows {
		return nil, ErrExportTooLarge
	}
	return out, nil
}
