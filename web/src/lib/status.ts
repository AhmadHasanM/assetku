import type { AssetStatus } from "./types"

/** Status tone; each maps to a CSS color token (--success, --info, ...) and a Badge variant. */
export type StatusTone = "success" | "info" | "warning" | "error" | "locked"

/** Asset status colors from spec §3. */
export const ASSET_STATUS_TONE: Record<AssetStatus, StatusTone> = {
  available: "success",
  in_use: "info",
  maintenance: "warning",
  damaged: "error",
  lost: "error",
  archived: "locked",
}
