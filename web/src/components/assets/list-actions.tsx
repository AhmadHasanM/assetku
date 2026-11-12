"use client"

import { useTranslations } from "next-intl"
import { Download, Upload } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import type { AssetFilters } from "@/lib/filters"

export interface ListActionProps {
  /** Active filters; Export acts on exactly this result set. */
  filters: AssetFilters
  /** "button" in the page header (md and up), "menu-item" inside the compact More menu. */
  as: "button" | "menu-item"
}

/**
 * Import and Export entry points of the list header. The page renders these
 * two components and nothing else, so a later task swaps the bodies without
 * touching the layout. Until then both are disabled.
 */
export function ImportAction({ as }: ListActionProps) {
  const t = useTranslations("assets")
  return as === "button" ? (
    <Button variant="secondary" disabled>
      <Upload />
      {t("import")}
    </Button>
  ) : (
    <DropdownMenuItem disabled>
      <Upload />
      {t("import")}
    </DropdownMenuItem>
  )
}

export function ExportAction({ as }: ListActionProps) {
  const t = useTranslations("assets")
  return as === "button" ? (
    <Button variant="secondary" disabled>
      <Download />
      {t("export")}
    </Button>
  ) : (
    <DropdownMenuItem disabled>
      <Download />
      {t("export")}
    </DropdownMenuItem>
  )
}
