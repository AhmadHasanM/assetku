-- name: GetAsset :one
SELECT a.id, a.name, a.asset_tag, a.category, a.manufacturer, a.model, a.serial_number,
       a.status, a.location_id, a.assigned_to, a.organization_id, a.supplier_id,
       a.order_number, a.purchase_date, a.purchase_cost, a.warranty_months, a.notes,
       a.created_at, a.updated_at,
       l.name AS location_name, o.name AS organization_name, s.name AS supplier_name
FROM assets a
LEFT JOIN locations l ON l.id = a.location_id
LEFT JOIN organizations o ON o.id = a.organization_id
LEFT JOIN suppliers s ON s.id = a.supplier_id
WHERE a.tenant_id = @tenant_id AND a.id = @id;

-- name: LockAsset :one
SELECT id FROM assets WHERE tenant_id = @tenant_id AND id = @id FOR UPDATE;

-- name: FindAssetIDByTag :one
SELECT id FROM assets WHERE tenant_id = @tenant_id AND lower(asset_tag) = lower(@asset_tag::text);

-- name: InsertAsset :one
-- ON CONFLICT on the tag index keeps a collision from aborting the caller's
-- transaction; no row returned means the tag is taken.
INSERT INTO assets (tenant_id, name, asset_tag, category, manufacturer, model, serial_number,
                    status, location_id, assigned_to, organization_id, supplier_id,
                    order_number, purchase_date, purchase_cost, warranty_months, notes)
VALUES (@tenant_id, @name, @asset_tag, @category, @manufacturer, @model, @serial_number,
        @status, @location_id, @assigned_to, @organization_id, @supplier_id,
        @order_number, @purchase_date, @purchase_cost, @warranty_months, @notes)
ON CONFLICT (tenant_id, lower(asset_tag)) DO NOTHING
RETURNING id;

-- name: UpdateAsset :execrows
UPDATE assets SET
    name = @name, asset_tag = @asset_tag, category = @category, manufacturer = @manufacturer,
    model = @model, serial_number = @serial_number, status = @status, location_id = @location_id,
    assigned_to = @assigned_to, organization_id = @organization_id, supplier_id = @supplier_id,
    order_number = @order_number, purchase_date = @purchase_date, purchase_cost = @purchase_cost,
    warranty_months = @warranty_months, notes = @notes, updated_at = now()
WHERE tenant_id = @tenant_id AND id = @id;

-- name: DeleteAsset :execrows
DELETE FROM assets WHERE tenant_id = @tenant_id AND id = @id;

-- name: AssigneeSuggestions :many
-- Distinct non-empty assignees containing q (case-insensitive), most used first.
SELECT assigned_to FROM assets
WHERE tenant_id = @tenant_id AND assigned_to <> '' AND assigned_to ILIKE '%' || @pattern::text || '%'
GROUP BY assigned_to
ORDER BY count(*) DESC, lower(assigned_to), assigned_to
LIMIT 10;
