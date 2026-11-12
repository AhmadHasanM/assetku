"use client"

import { useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { ASSET_STATUS_TONE } from "@/lib/status"
import type { AssetStatus } from "@/lib/types"

/** Pill with the status color at 20% fill (spec §3). */
export function StatusChip({ status }: { status: AssetStatus }) {
  const t = useTranslations("status")
  return <Badge variant={ASSET_STATUS_TONE[status] ?? "locked"}>{t(status)}</Badge>
}
