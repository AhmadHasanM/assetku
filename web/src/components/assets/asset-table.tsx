"use client"

import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react"
import { Card } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { AssetFilters, SortKey } from "@/lib/filters"
import type { AssetListItem } from "@/lib/types"
import { RowActions } from "./row-actions"
import { StatusChip } from "./status-chip"

const COLUMNS: readonly SortKey[] = ["asset_tag", "name", "category", "model", "serial_number", "status"]

interface Props {
  items: AssetListItem[]
  sort: AssetFilters["sort"]
  dir: AssetFilters["dir"]
  onSort: (key: SortKey) => void
  onDelete: (asset: Pick<AssetListItem, "id" | "name">) => void
}

/** Desktop list (md and up): sortable headers, clickable rows. */
export function AssetTable({ items, sort, dir, onSort, onDelete }: Props) {
  const t = useTranslations("assets")
  const tCat = useTranslations("category")
  const router = useRouter()

  return (
    <Card className="hidden gap-0 p-0 md:block">
      <Table>
        <TableHeader>
          <TableRow>
            {COLUMNS.map((key) => {
              const active = sort === key
              const Icon = !active ? ArrowUpDown : dir === "asc" ? ArrowUp : ArrowDown
              return (
                <TableHead key={key} aria-sort={active ? (dir === "asc" ? "ascending" : "descending") : "none"}>
                  <button
                    type="button"
                    onClick={() => onSort(key)}
                    aria-label={t("sortBy", { column: t(`columns.${key}`) })}
                    className="-mx-2 inline-flex min-h-11 items-center gap-1.5 rounded-md px-2 font-semibold hover:text-foreground focus-visible:glow-nebula-sm focus-visible:outline-none"
                  >
                    {t(`columns.${key}`)}
                    <Icon className={active ? "size-4 text-nebula-text" : "size-4 opacity-50"} aria-hidden />
                  </button>
                </TableHead>
              )
            })}
            <TableHead className="w-16 text-right">
              <span className="sr-only">{t("columns.actions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((a) => (
            <TableRow key={a.id} className="cursor-pointer" onClick={() => router.push(`/assets/${a.id}`)}>
              <TableCell className="font-mono text-sm whitespace-nowrap">{a.asset_tag}</TableCell>
              <TableCell className="max-w-64 truncate font-semibold" title={a.name}>
                {a.name}
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{tCat(a.category)}</TableCell>
              <TableCell className="max-w-40 truncate text-muted-foreground" title={a.model}>
                {a.model || "-"}
              </TableCell>
              <TableCell className="max-w-40 truncate font-mono text-sm text-muted-foreground" title={a.serial_number}>
                {a.serial_number || "-"}
              </TableCell>
              <TableCell>
                <StatusChip status={a.status} />
              </TableCell>
              <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                <RowActions asset={a} onDelete={onDelete} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  )
}
