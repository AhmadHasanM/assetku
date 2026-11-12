"use client"

import { useTranslations } from "next-intl"
import { Upload } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { openImportDialog } from "@/components/import/import-store"
import type { AssetFilters } from "@/lib/filters"
import { ExportMenu } from "./export-menu"

export interface ListActionProps {
  /** Active filters; Export acts on exactly this result set. */
  filters: AssetFilters
  /** "button" in the page header (md and up), "menu-item" inside the compact More menu. */
  as: "button" | "menu-item"
}

/**
 * Import and Export entry points of the list header. The import dialog itself
 * lives in the app shell (it outlives this page and the compact menu), so this
 * only opens it.
 */
export function ImportAction({ as }: ListActionProps) {
  const t = useTranslations("assets")
  return as === "button" ? (
    <Button variant="secondary" onClick={openImportDialog}>
      <Upload />
      {t("import")}
    </Button>
  ) : (
    <DropdownMenuItem onSelect={openImportDialog}>
      <Upload />
      {t("import")}
    </DropdownMenuItem>
  )
}

export function ExportAction({ filters, as }: ListActionProps) {
  return <ExportMenu filters={filters} as={as} />
}
