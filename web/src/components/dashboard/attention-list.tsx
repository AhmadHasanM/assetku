"use client"

import Link from "next/link"
import { ChevronRight, MapPin } from "lucide-react"
import { useLocale, useTranslations } from "next-intl"
import { StatusChip } from "@/components/assets/status-chip"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ATTENTION_STATUSES, DEFAULT_FILTERS, toSearchParams } from "@/lib/filters"
import { formatRelative } from "@/lib/format"
import type { DashboardData, Locale } from "@/lib/types"

const SHOWN = 6

/** "Perlu Perhatian": assets in maintenance, damaged, or lost; newest change first. */
export function AttentionList({ items }: { items: DashboardData["attention"] }) {
  const t = useTranslations("dashboard.attention")
  const locale = useLocale() as Locale
  const href = `/assets?${toSearchParams({ ...DEFAULT_FILTERS, status: [...ATTENTION_STATUSES] })}`

  return (
    <Card>
      <CardHeader className="flex items-center justify-between gap-3">
        <CardTitle className="font-heading text-xl">{t("title")}</CardTitle>
        {items.length > SHOWN && (
          <Link href={href} className="inline-flex min-h-11 items-center text-sm font-semibold text-nebula-text hover:underline">
            {t("viewAll", { count: items.length })}
          </Link>
        )}
      </CardHeader>
      <CardContent className="px-3 sm:px-4">
        {items.length === 0 ? (
          <p className="px-2 py-6 text-center text-muted-foreground">{t("empty")}</p>
        ) : (
          <ul className="flex flex-col">
            {items.slice(0, SHOWN).map((a) => (
              <li key={a.id} className="border-b last:border-b-0">
                <Link
                  href={`/assets/${a.id}`}
                  className="flex min-h-11 items-center gap-3 rounded-md px-2 py-3 outline-none transition-colors hover:bg-nebula/10 focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  <span className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <span className="font-mono text-sm text-nebula-text">{a.asset_tag}</span>
                      <StatusChip status={a.status} />
                    </span>
                    <span className="truncate font-semibold">{a.name}</span>
                    <span className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm text-muted-foreground">
                      <span className="inline-flex min-w-0 items-center gap-1">
                        <MapPin aria-hidden className="size-3.5 shrink-0" />
                        <span className="truncate">{a.location_name || t("noLocation")}</span>
                      </span>
                      <time dateTime={a.updated_at}>{formatRelative(a.updated_at, locale)}</time>
                    </span>
                  </span>
                  <ChevronRight aria-hidden className="size-4 shrink-0 text-subtle-foreground" />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
