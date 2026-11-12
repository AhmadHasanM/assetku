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

/** A referenced master data row ({id, name}). */
export interface MasterRef {
  id: string
  name: string
}

/** Body of POST /api/assets and PUT /api/assets/{id}. A missing or blank asset_tag means auto (create) or keep (update). */
export interface AssetInput {
  name: string
  asset_tag?: string
  category: string
  manufacturer: string
  model: string
  serial_number: string
  status: string
  location_id: string | null
  assigned_to: string
  organization_id: string | null
  supplier_id: string | null
  order_number: string
  /** yyyy-mm-dd */
  purchase_date: string | null
  /** Decimal string, e.g. "12500000" or "12500000.50". */
  purchase_cost: string | null
  warranty_months: number | null
  notes: string
}

/** GET /api/assets/{id} (also the POST/PUT response). */
export interface Asset extends Required<Omit<AssetInput, "category" | "status">> {
  id: string
  category: AssetCategory
  status: AssetStatus
  location: MasterRef | null
  organization: MasterRef | null
  supplier: MasterRef | null
  /** "12500000.00" */
  purchase_cost: string | null
  warranty_expires: string | null
  warranty_expired: boolean
  created_at: string
  updated_at: string
}

/** GET /api/imports/{id}. result is null until the job is done. */
export type ImportStatus = "queued" | "running" | "done" | "failed"

export interface ImportResultData {
  created: number
  updated: number
  unchanged: number
  skipped: number
  unknown_columns: string[]
  unmatched_organizations: { name: string; rows: number }[]
  unrecognized_values: { field: string; value: string; rows: number }[]
  /** reason: missing_name | duplicate_tag_in_file | invalid_value:<field> | error */
  skipped_rows: { row: number; reason: string }[]
  created_suppliers: string[]
  created_locations: string[]
}

export interface ImportJob {
  id: string
  status: ImportStatus
  total_rows: number
  processed_rows: number
  result: ImportResultData | null
  /** Failure code, e.g. server_restarted; empty unless failed. */
  error: string
}

/** One entry of GET /api/notifications; title/body are already in the user's language. */
export interface NotificationItem {
  id: number
  type: string
  title: string
  body: string
  asset_id: string | null
  read_at: string | null
  created_at: string
}

/** GET /api/notifications: the last 20 plus the total unread count. */
export interface NotificationList {
  unread: number
  items: NotificationItem[]
}

/** GET /api/dashboard. */
export interface DashboardData {
  summary: { total: number; available: number; in_use: number; needs_attention: number }
  attention: {
    id: string
    asset_tag: string
    name: string
    status: AssetStatus
    location_name: string | null
    updated_at: string
  }[]
  /** May include archived or lost assets (ruling T9). */
  upcoming_maintenance: {
    asset_id: string
    asset_tag: string
    name: string
    /** yyyy-mm-dd */
    next_date: string
    overdue: boolean
  }[]
  by_category: { key: AssetCategory; count: number }[]
  by_status: { key: AssetStatus; count: number }[]
}
