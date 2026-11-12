"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { Download, FileSpreadsheet, FileText, Loader2, Table } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { downloadBlob } from "@/components/assets/detail/download"
import { apiBlob } from "@/lib/api"
import { toSearchParams, type AssetFilters } from "@/lib/filters"
import { todayJakarta } from "@/lib/format"
import { useErrorMessage } from "@/lib/use-error-message"

/** Formats accepted by GET /api/assets/export. */
export const EXPORT_FORMATS = ["csv", "xlsx", "pdf"] as const
export type ExportFormat = (typeof EXPORT_FORMATS)[number]

const ICONS = { csv: Table, xlsx: FileSpreadsheet, pdf: FileText } as const

/** Export URL for the whole filtered result (every page, same sort). */
export function exportPath(format: ExportFormat, filters: AssetFilters): string {
  const qs = toSearchParams({ ...filters, page: 1 }).toString()
  return `/api/assets/export?format=${format}${qs ? `&${qs}` : ""}`
}

interface Props {
  filters: AssetFilters
  /** "button": its own dropdown; "menu-item": a submenu inside an existing DropdownMenu. */
  as: "button" | "menu-item"
}

/** CSV / Excel / PDF export of the active filters. */
export function ExportMenu({ filters, as }: Props) {
  const t = useTranslations("assets")
  const errorMessage = useErrorMessage()
  const [busy, setBusy] = useState(false)

  const run = async (format: ExportFormat) => {
    setBusy(true)
    // Inside the compact menu the item unmounts on select, so progress is a toast there.
    const loadingId = as === "menu-item" ? toast.loading(t("exportMenu.preparing")) : undefined
    try {
      const blob = await apiBlob(exportPath(format, filters))
      downloadBlob(blob, `assets-${todayJakarta().replaceAll("-", "")}.${format}`)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      if (loadingId !== undefined) toast.dismiss(loadingId)
      setBusy(false)
    }
  }

  const items = EXPORT_FORMATS.map((f) => {
    const Icon = ICONS[f]
    return (
      <DropdownMenuItem key={f} onSelect={() => void run(f)}>
        <Icon />
        {t(`exportMenu.${f}`)}
      </DropdownMenuItem>
    )
  })

  if (as === "menu-item") {
    return (
      <DropdownMenuSub>
        <DropdownMenuSubTrigger>
          <Download />
          {t("export")}
        </DropdownMenuSubTrigger>
        <DropdownMenuSubContent aria-label={t("exportMenu.label")}>{items}</DropdownMenuSubContent>
      </DropdownMenuSub>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" disabled={busy} aria-busy={busy}>
          {busy ? <Loader2 className="animate-spin" /> : <Download />}
          {t("export")}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" aria-label={t("exportMenu.label")}>
        {items}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
