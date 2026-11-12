import type { QueryClient } from "@tanstack/react-query"

/** Query-key roots of the master data lists (the lists show per-row asset counts). */
export const MASTER_DATA_KEYS = ["locations", "organizations", "suppliers"] as const

/**
 * After assets change (create, update, delete, move, import): the asset list,
 * summary and detail queries, the dashboard, and the master data lists whose
 * asset counts (and, after an import, rows) may have changed.
 */
export function invalidateAssetData(qc: QueryClient) {
  for (const key of ["assets", "dashboard", ...MASTER_DATA_KEYS]) qc.invalidateQueries({ queryKey: [key] })
}

/** After a master data row is renamed or deleted: its list, and the assets that show its name. */
export function invalidateMasterData(qc: QueryClient, kind: (typeof MASTER_DATA_KEYS)[number]) {
  qc.invalidateQueries({ queryKey: [kind] })
  qc.invalidateQueries({ queryKey: ["assets"] })
}
