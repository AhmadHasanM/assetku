"use client"

import Link from "next/link"
import { useTranslations } from "next-intl"
import { Eye, MoreHorizontal, Pencil, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { AssetListItem } from "@/lib/types"

interface Props {
  asset: Pick<AssetListItem, "id" | "name">
  onDelete: (asset: Pick<AssetListItem, "id" | "name">) => void
}

/** The per-row menu: View Details, Edit, Delete. */
export function RowActions({ asset, onDelete }: Props) {
  const t = useTranslations("assets")
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("rowActions", { name: asset.name })}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link href={`/assets/${asset.id}`}>
            <Eye />
            {t("viewDetails")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={`/assets/${asset.id}/edit`}>
            <Pencil />
            {t("edit")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => onDelete(asset)}>
          <Trash2 />
          {t("delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
