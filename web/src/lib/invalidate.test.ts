import { QueryClient } from "@tanstack/react-query"
import { describe, expect, it, vi } from "vitest"
import { invalidateAssetData, invalidateMasterData } from "./invalidate"

function invalidatedRoots(run: (qc: QueryClient) => void) {
  const qc = new QueryClient()
  const spy = vi.spyOn(qc, "invalidateQueries")
  run(qc)
  return spy.mock.calls.map(([filters]) => (filters?.queryKey as unknown[])[0]).sort()
}

describe("invalidate helpers", () => {
  it("asset changes refresh assets, dashboard and every master data list", () => {
    expect(invalidatedRoots(invalidateAssetData)).toEqual(
      ["assets", "dashboard", "locations", "organizations", "suppliers"].sort(),
    )
  })

  it("master data changes refresh that list and the assets", () => {
    expect(invalidatedRoots((qc) => invalidateMasterData(qc, "suppliers"))).toEqual(["assets", "suppliers"])
  })
})
