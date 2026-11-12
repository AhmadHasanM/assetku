import type { Locale } from "./types"

const EMPTY = "—"

const MONTHS: Record<Locale, readonly string[]> = {
  id: ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"],
  en: ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"],
}

/** "12500000.00" -> "Rp 12.500.000" (dot thousands, no decimals). Empty or invalid -> "—". */
export function formatRupiah(v: string | number | null | undefined): string {
  if (v === null || v === undefined || v === "") return EMPTY
  const n = typeof v === "number" ? v : Number(String(v).trim())
  if (!Number.isFinite(n)) return EMPTY
  const rounded = Math.round(Math.abs(n))
  const digits = rounded.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".")
  return `${n < 0 && rounded !== 0 ? "-" : ""}Rp ${digits}`
}

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/

const jakartaParts = new Intl.DateTimeFormat("en-US", {
  timeZone: "Asia/Jakarta",
  year: "numeric",
  month: "numeric",
  day: "numeric",
})

/** Date ("2026-08-11") or timestamp -> "dd MMM yyyy" in the given locale. Timestamps use Asia/Jakarta. */
export function formatDate(iso: string | null | undefined, locale: Locale): string {
  if (!iso) return EMPTY
  let y: number, m: number, d: number
  const dateOnly = DATE_ONLY.exec(iso)
  if (dateOnly) {
    y = Number(dateOnly[1])
    m = Number(dateOnly[2])
    d = Number(dateOnly[3])
  } else {
    const t = new Date(iso)
    if (Number.isNaN(t.getTime())) return EMPTY
    const parts = Object.fromEntries(
      jakartaParts.formatToParts(t).map((p) => [p.type, p.value]),
    )
    y = Number(parts.year)
    m = Number(parts.month)
    d = Number(parts.day)
  }
  const month = (MONTHS[locale] ?? MONTHS.id)[m - 1]
  if (!month || d < 1 || d > 31) return EMPTY
  return `${String(d).padStart(2, "0")} ${month} ${y}`
}

/** Today's date in Asia/Jakarta as yyyy-mm-dd (the API's "today"). */
export function todayJakarta(now: Date = new Date()): string {
  const parts = Object.fromEntries(jakartaParts.formatToParts(now).map((p) => [p.type, p.value]))
  return `${parts.year}-${String(parts.month).padStart(2, "0")}-${String(parts.day).padStart(2, "0")}`
}

const RELATIVE_STEPS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ["second", 60],
  ["minute", 60],
  ["hour", 24],
  ["day", 7],
  ["week", 4.35],
  ["month", 12],
  ["year", Infinity],
]

/** Timestamp -> "5 minutes ago" / "kemarin" in the given locale. Invalid -> "—". */
export function formatRelative(iso: string | null | undefined, locale: Locale, now: Date = new Date()): string {
  if (!iso) return EMPTY
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return EMPTY
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
  // Clock skew can put a fresh timestamp slightly in the future; show it as "now".
  let value = Math.min(0, (t - now.getTime()) / 1000)
  for (const [unit, size] of RELATIVE_STEPS) {
    if (Math.abs(value) < size) return rtf.format(Math.round(value), unit)
    value /= size
  }
  return EMPTY
}

/** Whole days from a to b, both yyyy-mm-dd (b later -> positive). NaN when either is malformed. */
export function daysBetween(a: string, b: string): number {
  const pa = DATE_ONLY.exec(a)
  const pb = DATE_ONLY.exec(b)
  if (!pa || !pb) return NaN
  const ua = Date.UTC(Number(pa[1]), Number(pa[2]) - 1, Number(pa[3]))
  const ub = Date.UTC(Number(pb[1]), Number(pb[2]) - 1, Number(pb[3]))
  return Math.round((ub - ua) / 86_400_000)
}
