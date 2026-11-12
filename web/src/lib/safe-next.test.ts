import { describe, expect, it } from "vitest"
import { safeAdminNext, safeNext } from "./safe-next"

describe("safeNext", () => {
  it("keeps relative paths with query", () => {
    expect(safeNext("/assets/x?y=1")).toBe("/assets/x?y=1")
  })
  it.each([
    ["//evil.com"],
    ["https://evil.com"],
    ["/\\evil"],
    ["\\\\evil.com"],
    ["assets/1"],
    [""],
    [null],
    ["/%5Cevil"],
    ["/\tevil"],
  ])("rejects %j", (input) => {
    expect(safeNext(input as string | null)).toBe("/dashboard")
  })
})

describe("safeAdminNext", () => {
  it("keeps admin panel paths", () => {
    expect(safeAdminNext("/admin/tenants/abc")).toBe("/admin/tenants/abc")
    expect(safeAdminNext("/admin")).toBe("/admin")
  })
  it.each([["//evil.com"], ["https://evil.com"], ["/dashboard"], ["/administrator"], ["/admin/login"], ["/admin/login?next=/x"], [null]])(
    "rejects %j",
    (input) => {
      expect(safeAdminNext(input as string | null)).toBe("/admin/tenants")
    },
  )
})
