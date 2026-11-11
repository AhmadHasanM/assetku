// Package asset implements tenant-scoped asset CRUD, asset tags and warranty.
package asset

import (
	"encoding/json"
	"slices"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Categories and Statuses are the valid enum codes, in display order.
var (
	Categories = []string{"it_equipment", "vehicle", "building_facility", "furniture", "electronics", "other"}
	Statuses   = []string{"available", "in_use", "maintenance", "damaged", "lost", "archived"}
)

// ValidCategory reports whether s is a category code.
func ValidCategory(s string) bool { return slices.Contains(Categories, s) }

// ValidStatus reports whether s is a status code.
func ValidStatus(s string) bool { return slices.Contains(Statuses, s) }

// Input holds every editable asset field. Update replaces all of them.
type Input struct {
	Name           string           `json:"name"`
	AssetTag       string           `json:"asset_tag"`
	Category       string           `json:"category"`
	Manufacturer   string           `json:"manufacturer"`
	Model          string           `json:"model"`
	SerialNumber   string           `json:"serial_number"`
	Status         string           `json:"status"`
	LocationID     *uuid.UUID       `json:"location_id"`
	AssignedTo     string           `json:"assigned_to"`
	OrganizationID *uuid.UUID       `json:"organization_id"`
	SupplierID     *uuid.UUID       `json:"supplier_id"`
	OrderNumber    string           `json:"order_number"`
	PurchaseDate   *civil.Date      `json:"purchase_date"`
	PurchaseCost   *decimal.Decimal `json:"purchase_cost"`
	WarrantyMonths *int             `json:"warranty_months"`
	Notes          string           `json:"notes"`
}

// Ref is a referenced master data row.
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Asset is a stored asset with resolved references and derived warranty info.
type Asset struct {
	ID uuid.UUID `json:"id"`
	Input
	Location        *Ref        `json:"location"`
	Organization    *Ref        `json:"organization"`
	Supplier        *Ref        `json:"supplier"`
	WarrantyExpires *civil.Date `json:"warranty_expires"`
	// WarrantyExpired is true when WarrantyExpires is before today in Asia/Jakarta.
	WarrantyExpired bool      `json:"warranty_expired"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// MarshalJSON renders purchase_cost as a string with two decimals ("12500000.00").
func (a Asset) MarshalJSON() ([]byte, error) {
	type plain Asset
	var cost *string
	if a.PurchaseCost != nil {
		s := a.PurchaseCost.StringFixed(2)
		cost = &s
	}
	return json.Marshal(struct {
		plain
		PurchaseCost *string `json:"purchase_cost"`
	}{plain(a), cost})
}
