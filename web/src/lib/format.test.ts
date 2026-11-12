import { describe, expect, it } from "vitest"
import { daysBetween, formatDate, formatRelative, formatRupiah, todayJakarta } from "./format"

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

describe("formatRelative", () => {
  const now = new Date("2026-11-12T10:00:00Z")
  it("formats recent times in the locale", () => {
    expect(formatRelative("2026-11-12T09:55:00Z", "en", now)).toBe("5 minutes ago")
    expect(formatRelative("2026-11-12T07:00:00Z", "id", now)).toBe("3 jam yang lalu")
  })
  it("uses words for yesterday", () => {
    expect(formatRelative("2026-11-11T10:00:00Z", "en", now)).toBe("yesterday")
    expect(formatRelative("2026-11-11T10:00:00Z", "id", now)).toBe("kemarin")
  })
  it("rounds before picking the unit", () => {
    expect(formatRelative("2026-11-12T09:00:24Z", "en", now)).toBe("1 hour ago")
    expect(formatRelative("2026-11-12T09:59:00.4Z", "en", now)).toBe("1 minute ago")
  })
  it("rolls up to months", () => {
    expect(formatRelative("2026-08-12T10:00:00Z", "en", now)).toBe("3 months ago")
  })
  it("treats future timestamps as now and invalid as an em dash", () => {
    expect(formatRelative("2026-11-12T10:00:05Z", "en", now)).toBe("now")
    expect(formatRelative("nope", "en", now)).toBe("—")
  })
})

describe("daysBetween", () => {
  it("counts calendar days", () => {
    expect(daysBetween("2026-11-12", "2026-11-19")).toBe(7)
    expect(daysBetween("2026-11-12", "2026-11-10")).toBe(-2)
    expect(daysBetween("2026-12-31", "2027-01-01")).toBe(1)
  })
  it("returns NaN for malformed input", () => {
    expect(daysBetween("x", "2026-11-12")).toBeNaN()
  })
})
