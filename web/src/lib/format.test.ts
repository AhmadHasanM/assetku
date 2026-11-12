import { describe, expect, it } from "vitest"
import { formatDate, formatRupiah, todayJakarta } from "./format"

describe("formatRupiah", () => {
  it("formats decimal strings with dot thousands and no decimals", () => {
    expect(formatRupiah("12500000.00")).toBe("Rp 12.500.000")
  })
  it("formats numbers", () => {
    expect(formatRupiah(750000)).toBe("Rp 750.000")
    expect(formatRupiah(0)).toBe("Rp 0")
  })
  it("returns an em dash for empty values", () => {
    expect(formatRupiah(null)).toBe("—")
    expect(formatRupiah("")).toBe("—")
    expect(formatRupiah("abc")).toBe("—")
  })
})

describe("formatDate", () => {
  it("uses Indonesian month abbreviations", () => {
    expect(formatDate("2026-08-11", "id")).toBe("11 Agu 2026")
    expect(formatDate("2026-10-01", "id")).toBe("01 Okt 2026")
    expect(formatDate("2026-12-25", "id")).toBe("25 Des 2026")
    expect(formatDate("2026-05-09", "id")).toBe("09 Mei 2026")
  })
  it("uses English month abbreviations", () => {
    expect(formatDate("2026-11-11", "en")).toBe("11 Nov 2026")
    expect(formatDate("2026-08-11", "en")).toBe("11 Aug 2026")
  })
  it("formats timestamps in Asia/Jakarta", () => {
    // 2026-11-10T20:00Z is 11 Nov 03:00 WIB.
    expect(formatDate("2026-11-10T20:00:00Z", "id")).toBe("11 Nov 2026")
  })
  it("returns an em dash for empty or invalid input", () => {
    expect(formatDate(null, "id")).toBe("—")
    expect(formatDate("not-a-date", "en")).toBe("—")
  })
})

describe("todayJakarta", () => {
  it("uses the Asia/Jakarta calendar day", () => {
    // 18:30 UTC is already 01:30 the next day in Jakarta (UTC+7).
    expect(todayJakarta(new Date("2026-11-30T18:30:00Z"))).toBe("2026-12-01")
    expect(todayJakarta(new Date("2026-11-30T16:59:00Z"))).toBe("2026-11-30")
  })
})
