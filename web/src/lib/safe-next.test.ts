import { describe, expect, it } from "vitest"
import { safeNext } from "./safe-next"

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
