"use client"

import { useInfiniteQuery } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import {
  ArrowRightLeft,
  CircleDot,
  FilePlus2,
  FileSpreadsheet,
  FileText,
  History,
  MapPin,
  PencilLine,
  Tag,
  Trash2,
  UserRound,
  Wrench,
  type LucideIcon,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { formatDate } from "@/lib/format"
import type { Locale } from "@/lib/types"
import { historyText, type HistoryPage, type Translate } from "./history-text"

const EVENT_ICON: Record<string, LucideIcon> = {
  asset_created: FilePlus2,
  asset_imported_created: FileSpreadsheet,
  asset_imported_updated: FileSpreadsheet,
  status_changed: CircleDot,
  location_changed: MapPin,
  assignee_changed: UserRound,
  details_updated: PencilLine,
  label_printed: Tag,
  bast_printed: FileText,
  maintenance_added: Wrench,
  maintenance_deleted: Trash2,
  movement_added: ArrowRightLeft,
  movement_deleted: Trash2,
}

const timeFormat = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Jakarta",
  hour: "2-digit",
  minute: "2-digit",
})

export const historyKey = (assetId: string) => ["assets", "detail", assetId, "history"] as const

/** Newest-first activity log, 20 per page with "Load more" (next_before). */
export function HistoryTab({ assetId }: { assetId: string }) {
  const t = useTranslations("assetDetail.historyTab")
  const tRoot = useTranslations() as unknown as Translate
  const locale = useLocale() as Locale
  const history = useInfiniteQuery({
    queryKey: historyKey(assetId),
    queryFn: ({ pageParam }) =>
      api<HistoryPage>(`/api/assets/${assetId}/history${pageParam ? `?before=${pageParam}` : ""}`),
    initialPageParam: 0,
    getNextPageParam: (last) => last.next_before ?? undefined,
  })

  if (history.isPending) {
    return (
      <div className="flex flex-col gap-3" aria-busy="true">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-16 rounded-lg" />
        ))}
      </div>
    )
  }
  if (history.isError) {
    return (
      <Card className="items-center gap-4 px-6 text-center">
        <p role="alert">{t("loadFailed")}</p>
        <Button variant="secondary" onClick={() => history.refetch()}>
          {tRoot("common.retry")}
        </Button>
      </Card>
    )
  }

  const items = history.data.pages.flatMap((p) => p.items)
  if (items.length === 0) {
    return (
      <Card className="items-center px-6 text-center text-muted-foreground">
        <p>{t("empty")}</p>
      </Card>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="py-2">
        <ol className="flex flex-col">
          {items.map((e) => {
            const Icon = EVENT_ICON[e.event] ?? History
            return (
              <li key={e.id} className="flex gap-3 border-b border-border px-4 py-3 last:border-b-0 sm:px-6">
                <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-full bg-nebula/15 text-nebula-text">
                  <Icon className="size-[18px]" aria-hidden />
                </span>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <p className="break-words">{historyText(e, tRoot, locale)}</p>
                  <p className="text-sm text-muted-foreground">
                    <time dateTime={e.created_at}>
                      {formatDate(e.created_at, locale)}, {timeFormat.format(new Date(e.created_at))}
                    </time>
                  </p>
                </div>
              </li>
            )
          })}
        </ol>
      </Card>
      {history.hasNextPage && (
        <Button
          variant="secondary"
          className="self-center"
          disabled={history.isFetchingNextPage}
          onClick={() => history.fetchNextPage()}
        >
          {history.isFetchingNextPage ? tRoot("common.loading") : t("loadMore")}
        </Button>
      )}
    </div>
  )
}
