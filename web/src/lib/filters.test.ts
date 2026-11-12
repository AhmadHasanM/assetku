import { describe, expect, it } from "vitest"
import {
  ATTENTION_STATUSES,
  DEFAULT_FILTERS,
  cardFromStatuses,
  cardStatuses,
  parseFilters,
  toSearchParams,
  updateFilters,
  type AssetFilters,
} from "./filters"

const LOC = "6f1f6c1e-2b3a-4c5d-8e9f-0a1b2c3d4e5f"

describe("filters", () => {
  it("parses an empty query to the defaults", () => {
    expect(parseFilters(new URLSearchParams())).toEqual(DEFAULT_FILTERS)
  })

  it("round-trips a full filter set", () => {
    const f: AssetFilters = {
      q: "laptop dell",
      category: ["it_equipment", "vehicle"],
      status: ["maintenance", "lost"],
      location_id: LOC,
      organization_id: LOC,
      sort: "name",
      dir: "asc",
      page: 3,
    }
    const params = toSearchParams(f)
    expect(params.get("category")).toBe("it_equipment,vehicle")
    expect(parseFilters(params)).toEqual(f)
  })

  it("only serialises non-default values", () => {
    expect(toSearchParams(DEFAULT_FILTERS).toString()).toBe("")
    expect(toSearchParams({ ...DEFAULT_FILTERS, page: 2 }).toString()).toBe("page=2")
    expect(toSearchParams({ ...DEFAULT_FILTERS, sort: "name" }).toString()).toBe("sort=name")
  })

  it("drops unknown enum values, bad sort keys and bad pages", () => {
    const f = parseFilters(new URLSearchParams("category=bogus,vehicle&status=x&sort=nope&dir=up&page=-4"))
    expect(f.category).toEqual(["vehicle"])
    expect(f.status).toEqual([])
    expect(f.sort).toBe("created_at")
    expect(f.dir).toBe("desc")
    expect(f.page).toBe(1)
  })

  it("maps status sets to summary cards", () => {
    expect(cardFromStatuses(["lost", "maintenance", "damaged"])).toBe("attention")
    expect(cardFromStatuses([])).toBe("total")
    expect(cardFromStatuses(["available"])).toBe("available")
    expect(cardFromStatuses(["in_use"])).toBe("in_use")
    expect(cardFromStatuses(["lost", "maintenance"])).toBeNull()
    expect(cardFromStatuses(["available", "in_use"])).toBeNull()
    expect(cardStatuses("attention")).toEqual([...ATTENTION_STATUSES])
    expect(cardStatuses("total")).toEqual([])
  })

  it("resets the page when a filter changes, but not when only the page changes", () => {
    const base: AssetFilters = { ...DEFAULT_FILTERS, page: 4 }
    expect(updateFilters(base, { q: "x" }).page).toBe(1)
    expect(updateFilters(base, { status: ["lost"] }).page).toBe(1)
    expect(updateFilters(base, { sort: "name", dir: "asc" }).page).toBe(1)
    expect(updateFilters(base, { page: 5 }).page).toBe(5)
  })
})
