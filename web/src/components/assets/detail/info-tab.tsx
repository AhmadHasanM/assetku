"use client"

import { useLocale, useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { formatDate, formatRupiah } from "@/lib/format"
import type { Asset, Locale } from "@/lib/types"
import { StatusChip } from "../status-chip"

type Row = [label: string, value: React.ReactNode]

function Section({ title, rows }: { title: string; rows: Row[] }) {
  return (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle>
          <h2 className="text-[22px] font-semibold">{title}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
          {rows.map(([label, value]) => (
            <div key={label} className="flex min-w-0 flex-col gap-0.5">
              <dt className="text-sm text-muted-foreground">{label}</dt>
              <dd className="break-words">{value}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}

/** The Information tab: General, Status & Placement, Purchase & Warranty, Notes. */
export function InfoTab({ asset }: { asset: Asset }) {
  const t = useTranslations("assetForm")
  const td = useTranslations("assetDetail.info")
  const tCat = useTranslations("category")
  const locale = useLocale() as Locale
  const f = (k: string) => t(`fields.${k}`)
  const or = (v: string | null | undefined) => (v && v.trim() ? v : "—")
  const mono = (v: string) => (v ? <span className="font-mono">{v}</span> : "—")

  return (
    <div className="flex flex-col gap-6">
      <Section
        title={t("sections.general")}
        rows={[
          [f("name"), asset.name],
          [f("asset_tag"), mono(asset.asset_tag)],
          [f("category"), tCat(asset.category)],
          [f("manufacturer"), or(asset.manufacturer)],
          [f("model"), or(asset.model)],
          [f("serial_number"), mono(asset.serial_number)],
        ]}
      />
      <Section
        title={t("sections.placement")}
        rows={[
          [f("status"), <StatusChip key="s" status={asset.status} />],
          [f("location"), or(asset.location?.name)],
          [f("assigned_to"), or(asset.assigned_to)],
          [f("organization"), or(asset.organization?.name)],
        ]}
      />
      <Section
        title={t("sections.purchase")}
        rows={[
          [f("supplier"), or(asset.supplier?.name)],
          [f("order_number"), or(asset.order_number)],
          [f("purchase_date"), formatDate(asset.purchase_date, locale)],
          [f("purchase_cost"), formatRupiah(asset.purchase_cost)],
          [f("warranty_months"), asset.warranty_months === null ? "—" : td("months", { count: asset.warranty_months })],
          [
            f("warranty_expires"),
            <span key="w" className="inline-flex flex-wrap items-center gap-2">
              {formatDate(asset.warranty_expires, locale)}
              {asset.warranty_expired && <Badge variant="error">{td("expired")}</Badge>}
            </span>,
          ],
        ]}
      />
      <Card className="gap-4">
        <CardHeader>
          <CardTitle>
            <h2 className="text-[22px] font-semibold">{t("sections.notes")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="break-words whitespace-pre-wrap">{or(asset.notes)}</p>
        </CardContent>
      </Card>
    </div>
  )
}
