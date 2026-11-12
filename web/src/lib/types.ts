export type Locale = "id" | "en"
export type Theme = "dark" | "light"

export const LOCALES: readonly Locale[] = ["id", "en"]
export const DEFAULT_LOCALE: Locale = "id"

/** Current user, as returned by the auth endpoints wrapped in {"user": Me}. */
export interface Me {
  id: string
  name: string
  email: string
  position: string
  locale: Locale
  theme: Theme
  tenant: { id: string; name: string }
}

export interface MeResponse {
  user: Me
}

/** Field-level error codes, e.g. {"new_password": "too_short"}. */
export type FieldErrors = Record<string, string>

/** Global API error envelope: {"error":{"code","message","fields"}}. */
export interface ApiErrorBody {
  error: {
    code: string
    message: string
    fields?: FieldErrors
  }
}

export const ASSET_CATEGORIES = [
  "it_equipment",
  "vehicle",
  "building_facility",
  "furniture",
  "electronics",
  "other",
] as const
export type AssetCategory = (typeof ASSET_CATEGORIES)[number]

export const ASSET_STATUSES = [
  "available",
  "in_use",
  "maintenance",
  "damaged",
  "lost",
  "archived",
] as const
export type AssetStatus = (typeof ASSET_STATUSES)[number]

export const MAINTENANCE_TYPES = ["preventive", "corrective", "inspection", "other"] as const
export type MaintenanceType = (typeof MAINTENANCE_TYPES)[number]

export const BAST_CONDITIONS = ["good", "new_sealed", "used_normal"] as const
export type BastCondition = (typeof BAST_CONDITIONS)[number]

/** One row of GET /api/assets. */
export interface AssetListItem {
  id: string
  asset_tag: string
  name: string
  category: AssetCategory
  model: string
  serial_number: string
  status: AssetStatus
  assigned_to: string
  location_name: string
  organization_name: string
}

export interface AssetListResponse {
  items: AssetListItem[]
  total: number
  page: number
  page_size: number
}
