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
