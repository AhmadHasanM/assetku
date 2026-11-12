import { formatDate } from "@/lib/format"
import type { Locale } from "@/lib/types"

/** One row of GET /api/assets/{id}/history. */
export interface HistoryEntry {
  id: number
  event: string
  actor_name: string
  detail: Record<string, unknown>
  created_at: string
}

export interface HistoryPage {
  items: HistoryEntry[]
  next_before: number | null
}

/** A root translator (useTranslations() or createTranslator without a namespace). */
export interface Translate {
  (key: string, values?: Record<string, string | number>): string
  has(key: string): boolean
}

export const HISTORY_EVENTS = [
  "asset_created",
  "asset_imported_created",
  "asset_imported_updated",
  "status_changed",
  "location_changed",
  "assignee_changed",
  "details_updated",
  "label_printed",
  "bast_printed",
  "maintenance_added",
  "maintenance_deleted",
  "movement_added",
  "movement_deleted",
] as const

const EMPTY = "—"
const str = (v: unknown): string => (typeof v === "string" ? v.trim() : "")
const orDash = (v: unknown) => str(v) || EMPTY

/** Translates "<ns>.<code>", falling back to the raw code for values the UI does not know. */
function label(t: Translate, ns: string, code: unknown): string {
  const c = str(code)
  if (!c) return EMPTY
  return t.has(`${ns}.${c}`) ? t(`${ns}.${c}`) : c
}

/** Asset field names as stored in history (location_id, ...) to their form labels. */
function fieldLabel(t: Translate, field: string): string {
  if (t.has(`history.fields.${field}`)) return t(`history.fields.${field}`)
  return label(t, "assetForm.fields", field)
}

/** A readable sentence for a history entry, from history.<event>. */
export function historyText(entry: HistoryEntry, t: Translate, locale: Locale = "id"): string {
  const d = entry.detail ?? {}
  const actor = str(entry.actor_name) || t("history.system")
  const fields = Array.isArray(d.fields) ? d.fields.map((f) => fieldLabel(t, String(f))).join(", ") : ""
  const date = typeof d.date === "string" ? formatDate(d.date, locale) : EMPTY

  let from = orDash(d.from)
  let to = orDash(d.to)
  if (entry.event === "status_changed") {
    from = label(t, "status", d.from)
    to = label(t, "status", d.to)
  }
  const values = {
    actor,
    from,
    to,
    fields,
    number: orDash(d.number),
    type: label(t, "maintenanceType", d.type),
    date,
    reason: str(d.reason),
  }
  const known = (HISTORY_EVENTS as readonly string[]).includes(entry.event)
  const text = t(known ? `history.${entry.event}` : "history.unknown", values)
  if (entry.event === "movement_added" && values.reason) return `${text} ${t("history.reason", values)}`
  return text
}
