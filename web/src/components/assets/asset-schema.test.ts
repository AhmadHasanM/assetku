import { describe, expect, it } from "vitest"
import type { Asset } from "@/lib/types"
import {
  EMPTY_ASSET,
  assetSchema,
  fromAsset,
  parseCost,
  toAssetInput,
  warrantyExpiresPreview,
  type AssetFormValues,
} from "./asset-schema"

const valid: AssetFormValues = { ...EMPTY_ASSET, name: "Laptop", category: "it_equipment" }

function fieldErrors(values: AssetFormValues): Record<string, string> {
  const r = assetSchema.safeParse(values)
  if (r.success) return {}
  return Object.fromEntries(r.error.issues.map((i) => [i.path.join("."), i.message]))
}

describe("assetSchema", () => {
  it("accepts the minimal valid form", () => {
    expect(fieldErrors(valid)).toEqual({})
  })

  it("defaults status to available", () => {
    expect(EMPTY_ASSET.status).toBe("available")
  })

  it("rejects an empty or blank name", () => {
    expect(fieldErrors({ ...valid, name: "" })).toEqual({ name: "required" })
    expect(fieldErrors({ ...valid, name: "   " })).toEqual({ name: "required" })
  })

  it("requires category and status, and rejects unknown codes", () => {
    expect(fieldErrors({ ...valid, category: "" })).toEqual({ category: "required" })
    expect(fieldErrors({ ...valid, category: "boat" })).toEqual({ category: "invalid" })
    expect(fieldErrors({ ...valid, status: "" })).toEqual({ status: "required" })
    expect(fieldErrors({ ...valid, status: "gone" })).toEqual({ status: "invalid" })
  })

  it("validates purchase_date as a real yyyy-mm-dd date", () => {
    expect(fieldErrors({ ...valid, purchase_date: "2026-02-28" })).toEqual({})
    expect(fieldErrors({ ...valid, purchase_date: "2026-02-30" })).toEqual({ purchase_date: "invalid" })
    expect(fieldErrors({ ...valid, purchase_date: "28/02/2026" })).toEqual({ purchase_date: "invalid" })
  })

  it("keeps numbers at or above zero and inside the API limits", () => {
    expect(fieldErrors({ ...valid, purchase_cost: "0", warranty_months: "0" })).toEqual({})
    expect(fieldErrors({ ...valid, purchase_cost: "-1" })).toEqual({ purchase_cost: "invalid" })
    expect(fieldErrors({ ...valid, purchase_cost: "abc" })).toEqual({ purchase_cost: "invalid" })
    expect(fieldErrors({ ...valid, purchase_cost: "99999999999999" })).toEqual({})
    expect(fieldErrors({ ...valid, purchase_cost: "100000000000000" })).toEqual({ purchase_cost: "invalid" })
    expect(fieldErrors({ ...valid, warranty_months: "-1" })).toEqual({ warranty_months: "invalid" })
    expect(fieldErrors({ ...valid, warranty_months: "1.5" })).toEqual({ warranty_months: "invalid" })
    expect(fieldErrors({ ...valid, warranty_months: "1200" })).toEqual({})
    expect(fieldErrors({ ...valid, warranty_months: "1201" })).toEqual({ warranty_months: "invalid" })
  })
})

describe("parseCost", () => {
  it("accepts plain numbers and Rupiah formatting", () => {
    expect(parseCost("12500000")).toBe("12500000")
    expect(parseCost("Rp 12.500.000")).toBe("12500000")
    expect(parseCost("12.500.000,50")).toBe("12500000.50")
    expect(parseCost("12500000.5")).toBe("12500000.5")
    expect(parseCost("0012")).toBe("12")
    expect(parseCost("")).toBeNull()
    expect(parseCost("1e5")).toBeNull()
  })
})

describe("warrantyExpiresPreview", () => {
  it("clamps to the end of the month like the API", () => {
    expect(warrantyExpiresPreview("2026-01-31", 1)).toBe("2026-02-28")
    expect(warrantyExpiresPreview("2028-01-31", 1)).toBe("2028-02-29")
    expect(warrantyExpiresPreview("2026-03-31", 1)).toBe("2026-04-30")
    expect(warrantyExpiresPreview("2026-01-15", 12)).toBe("2027-01-15")
    expect(warrantyExpiresPreview("2026-11-30", 3)).toBe("2027-02-28")
    expect(warrantyExpiresPreview("2026-05-10", 0)).toBe("2026-05-10")
  })

  it("returns null when either part is missing or invalid", () => {
    expect(warrantyExpiresPreview(undefined, 12)).toBeNull()
    expect(warrantyExpiresPreview("2026-01-31", undefined)).toBeNull()
    expect(warrantyExpiresPreview("", 12)).toBeNull()
    expect(warrantyExpiresPreview("2026-02-30", 12)).toBeNull()
    expect(warrantyExpiresPreview("2026-01-31", Number.NaN)).toBeNull()
  })
})

describe("toAssetInput", () => {
  it("omits a blank tag and sends nulls for empty optional values", () => {
    const body = toAssetInput({ ...valid, name: "  Laptop  ", asset_tag: "  " })
    expect(body).not.toHaveProperty("asset_tag")
    expect(body).toMatchObject({
      name: "Laptop",
      category: "it_equipment",
      status: "available",
      location_id: null,
      organization_id: null,
      supplier_id: null,
      purchase_date: null,
      purchase_cost: null,
      warranty_months: null,
    })
  })

  it("maps master refs, numbers and dates", () => {
    const body = toAssetInput({
      ...valid,
      asset_tag: " AST-1 ",
      location: { id: "11111111-1111-1111-1111-111111111111", name: "HQ" },
      purchase_date: "2026-01-31",
      purchase_cost: "Rp 12.500.000",
      warranty_months: "12",
    })
    expect(body).toMatchObject({
      asset_tag: "AST-1",
      location_id: "11111111-1111-1111-1111-111111111111",
      purchase_date: "2026-01-31",
      purchase_cost: "12500000",
      warranty_months: 12,
    })
  })

  it("omits an unchanged tag on edit but sends a changed one", () => {
    expect(toAssetInput({ ...valid, asset_tag: "AST-1" }, "AST-1")).not.toHaveProperty("asset_tag")
    expect(toAssetInput({ ...valid, asset_tag: "AST-2" }, "AST-1")).toHaveProperty("asset_tag", "AST-2")
    expect(toAssetInput({ ...valid, asset_tag: "" }, "AST-1")).not.toHaveProperty("asset_tag")
  })
})

describe("fromAsset", () => {
  it("turns an API asset into form values", () => {
    const asset = {
      id: "a1",
      name: "Laptop",
      asset_tag: "AST-1",
      category: "it_equipment",
      manufacturer: "",
      model: "",
      serial_number: "",
      status: "in_use",
      location_id: "l1",
      location: { id: "l1", name: "HQ" },
      assigned_to: "Budi",
      organization_id: null,
      organization: null,
      supplier_id: null,
      supplier: null,
      order_number: "",
      purchase_date: "2026-01-31",
      purchase_cost: "12500000.00",
      warranty_months: 12,
      warranty_expires: "2027-01-31",
      warranty_expired: false,
      notes: "",
      created_at: "",
      updated_at: "",
    } satisfies Asset
    expect(fromAsset(asset)).toMatchObject({
      status: "in_use",
      location: { id: "l1", name: "HQ" },
      organization: null,
      purchase_cost: "12500000",
      warranty_months: "12",
      purchase_date: "2026-01-31",
    })
    expect(fromAsset({ ...asset, purchase_cost: "10.50" }).purchase_cost).toBe("10.50")
  })
})
