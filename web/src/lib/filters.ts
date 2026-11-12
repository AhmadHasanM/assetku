import { ASSET_CATEGORIES, ASSET_STATUSES, type AssetStatus } from "./types"

/** Statuses counted by the "Needs Attention" summary card. */
export const ATTENTION_STATUSES = ["maintenance", "damaged", "lost"] as const

/** Sort keys accepted by GET /api/assets. */
export const SORT_KEYS = [
  "asset_tag",
  "name",
  "category",
  "model",
  "serial_number",
  "status",
  "created_at",
] as const
export type SortKey = (typeof SORT_KEYS)[number]

export interface AssetFilters {
  q: string
  category: string[]
  status: string[]
  location_id?: string
  organization_id?: string
  sort: SortKey
  dir: "asc" | "desc"
  page: number
}

export const DEFAULT_FILTERS: AssetFilters = {
  q: "",
  category: [],
  status: [],
  sort: "created_at",
  dir: "desc",
  page: 1,
}

export type SummaryCard = "total" | "available" | "in_use" | "attention"

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function parseEnums(raw: string | null, valid: readonly string[]): string[] {
  if (!raw) return []
  const out: string[] = []
  for (const part of raw.split(",")) {
    const v = part.trim()
    if (valid.includes(v) && !out.includes(v)) out.push(v)
  }
  return out
}

/** Reads list filters from URL params; unknown or malformed values fall back to defaults. */
export function parseFilters(params: URLSearchParams): AssetFilters {
  const sort = params.get("sort")
  const dir = params.get("dir")
  const page = Number(params.get("page"))
  const loc = params.get("location_id")
  const org = params.get("organization_id")
  return {
    q: params.get("q") ?? "",
    category: parseEnums(params.get("category"), ASSET_CATEGORIES),
    status: parseEnums(params.get("status"), ASSET_STATUSES),
    location_id: loc && UUID.test(loc) ? loc : undefined,
    organization_id: org && UUID.test(org) ? org : undefined,
    sort: (SORT_KEYS as readonly string[]).includes(sort ?? "") ? (sort as SortKey) : DEFAULT_FILTERS.sort,
    dir: dir === "asc" || dir === "desc" ? dir : DEFAULT_FILTERS.dir,
    page: Number.isInteger(page) && page >= 1 ? page : 1,
  }
}

/** Serialises filters, omitting every value that equals its default. */
export function toSearchParams(f: AssetFilters): URLSearchParams {
  const p = new URLSearchParams()
  if (f.q) p.set("q", f.q)
  if (f.category.length) p.set("category", f.category.join(","))
  if (f.status.length) p.set("status", f.status.join(","))
  if (f.location_id) p.set("location_id", f.location_id)
  if (f.organization_id) p.set("organization_id", f.organization_id)
  if (f.sort !== DEFAULT_FILTERS.sort) p.set("sort", f.sort)
  if (f.dir !== DEFAULT_FILTERS.dir) p.set("dir", f.dir)
  if (f.page !== 1) p.set("page", String(f.page))
  return p
}

/** Applies a change; any change other than the page itself returns to page 1. */
export function updateFilters(current: AssetFilters, patch: Partial<AssetFilters>): AssetFilters {
  return { ...current, page: 1, ...patch }
}

/** Summary card matching the status filter, or null when no card represents it. */
export function cardFromStatuses(status: readonly string[]): SummaryCard | null {
  if (status.length === 0) return "total"
  const set = new Set(status)
  if (set.size === 1) {
    if (set.has("available")) return "available"
    if (set.has("in_use")) return "in_use"
  }
  if (set.size === ATTENTION_STATUSES.length && ATTENTION_STATUSES.every((s) => set.has(s))) return "attention"
  return null
}

/** Status filter a summary card sets. */
export function cardStatuses(card: SummaryCard): AssetStatus[] {
  switch (card) {
    case "available":
      return ["available"]
    case "in_use":
      return ["in_use"]
    case "attention":
      return [...ATTENTION_STATUSES]
    default:
      return []
  }
}
