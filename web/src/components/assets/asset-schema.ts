import { z } from "zod"
import { ASSET_CATEGORIES, ASSET_STATUSES, type Asset, type AssetInput, type MasterRef } from "@/lib/types"

// Mirrors api/internal/asset/service.go: purchase_cost < 1e14 (numeric(16,2)), warranty 0..1200 months.
const MAX_COST_INTEGER_DIGITS = 14
const MAX_WARRANTY_MONTHS = 1200

const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/

function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0)).getUTCDate()
}

function parseIsoDate(s: string | undefined): { y: number; m: number; d: number } | null {
  const match = s ? ISO_DATE.exec(s) : null
  if (!match) return null
  const [y, m, d] = [Number(match[1]), Number(match[2]), Number(match[3])]
  if (m < 1 || m > 12 || d < 1 || d > daysInMonth(y, m)) return null
  return { y, m, d }
}

const pad = (n: number, w = 2) => String(n).padStart(w, "0")

/**
 * purchase_date + months, clamping the day to the end of the target month
 * (31 Jan + 1 = 28/29 Feb), exactly like asset.AddMonthsClamped. Null when
 * either part is missing or invalid.
 */
export function warrantyExpiresPreview(purchase_date?: string, months?: number): string | null {
  const date = parseIsoDate(purchase_date)
  if (!date || months === undefined || !Number.isInteger(months)) return null
  const total = date.y * 12 + (date.m - 1) + months
  const year = Math.floor(total / 12)
  const month = total - year * 12 + 1
  const day = Math.min(date.d, daysInMonth(year, month))
  return `${pad(year, 4)}-${pad(month)}-${pad(day)}`
}

/**
 * Parses a typed cost into the decimal string the API accepts. Takes plain
 * numbers ("12500000", "12500000.5") and Rupiah formatting ("Rp 12.500.000",
 * "12.500.000,50"). Null when empty, malformed, negative, or >= 1e14.
 */
export function parseCost(input: string): string | null {
  const s = input.trim().replace(/^rp\.?\s*/i, "").replace(/\s/g, "")
  let int: string
  let frac = ""
  let m: RegExpExecArray | null
  if ((m = /^(\d{1,3}(?:\.\d{3})+)(?:,(\d{1,2}))?$/.exec(s))) {
    int = m[1].replaceAll(".", "")
    frac = m[2] ?? ""
  } else if ((m = /^(\d+)(?:[.,](\d{1,2}))?$/.exec(s))) {
    int = m[1]
    frac = m[2] ?? ""
  } else {
    return null
  }
  int = int.replace(/^0+(?=\d)/, "")
  if (int.length > MAX_COST_INTEGER_DIGITS) return null
  return frac ? `${int}.${frac}` : int
}

function parseMonths(input: string): number | null {
  const s = input.trim()
  if (!/^\d+$/.test(s)) return null
  const n = Number(s)
  return n <= MAX_WARRANTY_MONTHS ? n : null
}

const masterRef = z.object({ id: z.string(), name: z.string() }).nullable()

const requiredEnum = (codes: readonly string[]) =>
  z
    .string()
    .refine((v) => v !== "", { error: "required" })
    .refine((v) => v === "" || codes.includes(v), { error: "invalid" })

/** Client-side validation matching the API rules. Error messages are field error codes. */
export const assetSchema = z.object({
  name: z.string().refine((v) => v.trim() !== "", { error: "required" }),
  asset_tag: z.string(),
  category: requiredEnum(ASSET_CATEGORIES),
  manufacturer: z.string(),
  model: z.string(),
  serial_number: z.string(),
  status: requiredEnum(ASSET_STATUSES),
  location: masterRef,
  assigned_to: z.string(),
  organization: masterRef,
  supplier: masterRef,
  order_number: z.string(),
  purchase_date: z.string().refine((v) => v === "" || parseIsoDate(v) !== null, { error: "invalid" }),
  purchase_cost: z.string().refine((v) => v.trim() === "" || parseCost(v) !== null, { error: "invalid" }),
  warranty_months: z.string().refine((v) => v.trim() === "" || parseMonths(v) !== null, { error: "invalid" }),
  notes: z.string(),
})

export type AssetFormValues = z.infer<typeof assetSchema>
export type AssetFormField = keyof AssetFormValues

export const EMPTY_ASSET: AssetFormValues = {
  name: "",
  asset_tag: "",
  category: "",
  manufacturer: "",
  model: "",
  serial_number: "",
  status: "available",
  location: null,
  assigned_to: "",
  organization: null,
  supplier: null,
  order_number: "",
  purchase_date: "",
  purchase_cost: "",
  warranty_months: "",
  notes: "",
}

/** API field name -> form field, for mapping fields.* errors. */
export const API_FIELD_TO_FORM: Record<string, AssetFormField> = {
  name: "name",
  asset_tag: "asset_tag",
  category: "category",
  manufacturer: "manufacturer",
  model: "model",
  serial_number: "serial_number",
  status: "status",
  location_id: "location",
  assigned_to: "assigned_to",
  organization_id: "organization",
  supplier_id: "supplier",
  order_number: "order_number",
  purchase_date: "purchase_date",
  purchase_cost: "purchase_cost",
  warranty_months: "warranty_months",
  notes: "notes",
}

/** "12500000.00" -> "12500000"; other decimals are kept as is. */
function costForInput(cost: string | null): string {
  if (!cost) return ""
  return cost.replace(/\.0+$/, "")
}

/** Form values for editing an existing asset. */
export function fromAsset(a: Asset): AssetFormValues {
  const ref = (r: MasterRef | null) => (r ? { id: r.id, name: r.name } : null)
  return {
    name: a.name,
    asset_tag: a.asset_tag,
    category: a.category,
    manufacturer: a.manufacturer,
    model: a.model,
    serial_number: a.serial_number,
    status: a.status,
    location: ref(a.location),
    assigned_to: a.assigned_to,
    organization: ref(a.organization),
    supplier: ref(a.supplier),
    order_number: a.order_number,
    purchase_date: a.purchase_date ?? "",
    purchase_cost: costForInput(a.purchase_cost),
    warranty_months: a.warranty_months === null ? "" : String(a.warranty_months),
    notes: a.notes,
  }
}

/**
 * Request body for create/update. asset_tag is omitted when blank (create:
 * auto-generate) or, on edit, when it equals originalTag (the API keeps it).
 */
export function toAssetInput(v: AssetFormValues, originalTag?: string): AssetInput {
  const body: AssetInput = {
    name: v.name.trim(),
    category: v.category,
    manufacturer: v.manufacturer.trim(),
    model: v.model.trim(),
    serial_number: v.serial_number.trim(),
    status: v.status,
    location_id: v.location?.id ?? null,
    assigned_to: v.assigned_to.trim(),
    organization_id: v.organization?.id ?? null,
    supplier_id: v.supplier?.id ?? null,
    order_number: v.order_number.trim(),
    purchase_date: v.purchase_date || null,
    purchase_cost: v.purchase_cost.trim() ? parseCost(v.purchase_cost) : null,
    warranty_months: v.warranty_months.trim() ? parseMonths(v.warranty_months) : null,
    notes: v.notes,
  }
  const tag = v.asset_tag.trim()
  if (tag && tag !== originalTag) body.asset_tag = tag
  return body
}
