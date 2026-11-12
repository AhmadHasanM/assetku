"use client"

import { useSyncExternalStore } from "react"
import Link from "next/link"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { FileText, MoreHorizontal, Pencil, Tag, Trash2 } from "lucide-react"
import { QRCodeSVG } from "qrcode.react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { apiBlob } from "@/lib/api"
import type { Asset } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { StatusChip } from "../status-chip"
import { downloadBlob } from "./download"

const noSubscribe = () => () => {}

/** window.location.origin on the client, "" during SSR (the QR renders after hydration). */
function useOrigin(): string {
  return useSyncExternalStore(
    noSubscribe,
    () => window.location.origin,
    () => "",
  )
}

interface Props {
  asset: Asset
  onBast: () => void
  onDelete: () => void
}

/** QR code, identity, placement and the asset actions (collapsed into a menu below 768px). */
export function AssetHeader({ asset, onBast, onDelete }: Props) {
  const t = useTranslations("assetDetail")
  const tCat = useTranslations("category")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const origin = useOrigin()

  const label = useMutation({
    mutationFn: () => apiBlob(`/api/assets/${asset.id}/label`, { method: "POST" }),
    onSuccess: (blob) => {
      downloadBlob(blob, `label-${asset.asset_tag}.pdf`)
      qc.invalidateQueries({ queryKey: ["assets", "detail", asset.id, "history"] })
      toast.success(t("actions.labelDone"))
    },
    onError: (err) => toast.error(errorMessage(err)),
  })
  const labelText = label.isPending ? t("actions.generating") : t("actions.label")
  const editHref = `/assets/${asset.id}/edit`

  const facts: [string, React.ReactNode][] = [
    [t("header.category"), tCat(asset.category)],
    [t("header.serial"), asset.serial_number ? <span className="font-mono">{asset.serial_number}</span> : "—"],
    [t("header.manufacturer"), asset.manufacturer || "—"],
    [t("header.model"), asset.model || "—"],
    [t("header.location"), asset.location?.name || "—"],
    [t("header.assignee"), asset.assigned_to || "—"],
  ]

  return (
    <Card className="gap-0 p-4 sm:p-6">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
        <div className="flex items-start justify-between gap-3">
          <div
            className="flex size-[120px] shrink-0 items-center justify-center rounded-md bg-white p-2.5 sm:size-[140px]"
            role="img"
            aria-label={t("qrLabel", { tag: asset.asset_tag })}
          >
            {origin && (
              <QRCodeSVG
                value={`${origin}/assets/${asset.id}`}
                size={256}
                marginSize={0}
                bgColor="#FFFFFF"
                fgColor="#000000"
                className="size-full"
              />
            )}
          </div>
          {/* Below 768px every action lives in this menu. */}
          <div className="md:hidden">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="secondary" size="icon" aria-label={t("actions.more")}>
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-52">
                <DropdownMenuItem disabled={label.isPending} onSelect={() => label.mutate()}>
                  <Tag />
                  {labelText}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={onBast}>
                  <FileText />
                  {t("actions.bast")}
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link href={editHref}>
                    <Pencil />
                    {t("actions.edit")}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" onSelect={onDelete}>
                  <Trash2 />
                  {t("actions.delete")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-3">
          <div className="flex flex-col gap-2">
            {/* The page title, set at the h2 size of the type scale. */}
            <h1 className="text-[28px] leading-tight break-words">{asset.name}</h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <StatusChip status={asset.status} />
              <span className="font-mono text-sm font-bold">{asset.asset_tag}</span>
            </div>
          </div>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 min-[480px]:grid-cols-2 lg:grid-cols-3">
            {facts.map(([term, value]) => (
              <div key={term} className="flex min-w-0 flex-col">
                <dt className="text-tiny text-subtle-foreground">{term}</dt>
                <dd className="text-sm break-words">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>

      <div className="mt-5 hidden flex-wrap justify-end gap-2 md:flex">
        <Button variant="secondary" disabled={label.isPending} onClick={() => label.mutate()}>
          <Tag />
          {labelText}
        </Button>
        <Button variant="secondary" onClick={onBast}>
          <FileText />
          {t("actions.bast")}
        </Button>
        <Button asChild variant="secondary">
          <Link href={editHref}>
            <Pencil />
            {t("actions.edit")}
          </Link>
        </Button>
        <Button variant="destructive" onClick={onDelete}>
          <Trash2 />
          {t("actions.delete")}
        </Button>
      </div>
    </Card>
  )
}
