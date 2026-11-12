import { describe, expect, it } from "vitest"
import { createTranslator } from "next-intl"
import id from "@/messages/id.json"
import en from "@/messages/en.json"
import { historyText, type HistoryEntry, type Translate } from "./history-text"

const tId = createTranslator({ locale: "id", messages: id }) as unknown as Translate
const tEn = createTranslator({ locale: "en", messages: en }) as unknown as Translate

const entry = (event: string, detail: Record<string, unknown>, actor = "Budi"): HistoryEntry => ({
  id: 1,
  event,
  actor_name: actor,
  detail,
  created_at: "2026-11-12T03:00:00Z",
})

describe("historyText", () => {
  it("status_changed uses translated status labels", () => {
    expect(historyText(entry("status_changed", { from: "available", to: "in_use" }), tId, "id")).toBe(
      "Budi mengubah status dari Tersedia ke Digunakan",
    )
  })

  it("movement_added names both locations", () => {
    const text = historyText(
      entry("movement_added", { from: "Kantor Jakarta", to: "Gudang Bekasi", reason: "" }),
      tId,
      "id",
    )
    expect(text).toContain("Kantor Jakarta")
    expect(text).toContain("Gudang Bekasi")
  })

  it("movement_added appends the reason when given", () => {
    const text = historyText(entry("movement_added", { from: "A", to: "B", reason: "Renovasi" }), tEn, "en")
    expect(text).toContain("Renovasi")
  })

  it("details_updated translates field names", () => {
    const text = historyText(entry("details_updated", { fields: ["name", "supplier_id", "purchase_cost"] }), tEn, "en")
    expect(text).toBe("Budi updated Asset Name, Supplier, Purchase Cost")
  })

  it("bast_printed includes the number and maintenance dates are formatted", () => {
    expect(historyText(entry("bast_printed", { number: "BAST-AST-1-20261111" }), tEn, "en")).toContain(
      "BAST-AST-1-20261111",
    )
    expect(historyText(entry("maintenance_added", { type: "preventive", date: "2026-08-11" }), tId, "id")).toContain(
      "11 Agu 2026",
    )
  })

  it("an empty location shows a dash and an unknown event falls back", () => {
    expect(historyText(entry("location_changed", { from: "", to: "Gudang" }), tEn, "en")).toContain("—")
    expect(historyText(entry("something_new", {}), tEn, "en")).toContain("Budi")
  })
})
