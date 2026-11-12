"use client"

import Link from "next/link"
import { useTranslations } from "next-intl"
import { Card } from "@/components/ui/card"
import type { AssetListItem } from "@/lib/types"
import { RowActions } from "./row-actions"
import { StatusChip } from "./status-chip"

interface Props {
  items: AssetListItem[]
  onDelete: (asset: Pick<AssetListItem, "id" | "name">) => void
}

/** Mobile list (below md): one card per asset. */
export function AssetCardList({ items, onDelete }: Props) {
  const tCat = useTranslations("category")
  return (
    <ul className="flex flex-col gap-3 md:hidden">
      {items.map((a) => (
        <li key={a.id}>
          <Card className="gap-2 p-4">
            <div className="flex items-start justify-between gap-2">
              <Link href={`/assets/${a.id}`} className="min-w-0 flex-1 rounded-md focus-visible:glow-nebula-sm focus-visible:outline-none">
                <p className="font-mono text-sm text-muted-foreground">{a.asset_tag}</p>
                <p className="font-heading text-lg font-semibold break-words">{a.name}</p>
              </Link>
              <RowActions asset={a} onDelete={onDelete} />
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <StatusChip status={a.status} />
              <span className="text-sm text-muted-foreground">{tCat(a.category)}</span>
            </div>
          </Card>
        </li>
      ))}
    </ul>
  )
}
