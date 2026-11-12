"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { useFormatter, useTranslations } from "next-intl"
import { ChevronLeft, ChevronRight, MoreHorizontal, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import {
  DEFAULT_FILTERS,
  cardFromStatuses,
  parseFilters,
  toSearchParams,
  updateFilters,
  type AssetFilters,
  type SortKey,
} from "@/lib/filters"
import type { AssetListItem, AssetListResponse } from "@/lib/types"
import { AssetCardList } from "./asset-card-list"
import { AssetFilterBar } from "./asset-filters"
import { AssetTable } from "./asset-table"
import { DeleteAssetDialog } from "./delete-asset-dialog"
import { ExportAction, ImportAction } from "./list-actions"
import { SummaryCards, type AssetSummary } from "./summary-cards"

/** Page numbers to show: first, last, and a window around the current page; null marks a gap. */
export function pageWindow(page: number, pages: number): (number | null)[] {
  const keep = new Set([1, pages, page - 1, page, page + 1].filter((p) => p >= 1 && p <= pages))
  const out: (number | null)[] = []
  let prev = 0
  for (const p of [...keep].sort((a, b) => a - b)) {
    if (p - prev > 1) out.push(null)
    out.push(p)
    prev = p
  }
  return out
}

/** The /assets page body. Filters, sort, and page live in the URL (replaced, not pushed). */
export function AssetListView() {
  const t = useTranslations("assets")
  const tc = useTranslations("common")
  const format = useFormatter()
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const [deleting, setDeleting] = useState<Pick<AssetListItem, "id" | "name"> | null>(null)

  const query = searchParams.toString()
  const filters = useMemo(() => parseFilters(new URLSearchParams(query)), [query])

  // Debounced search can fire after other filters changed; always patch the latest state.
  const latest = useRef(filters)
  useEffect(() => {
    latest.current = filters
  }, [filters])

  const navigate = useCallback(
    (next: AssetFilters) => {
      latest.current = next
      const qs = toSearchParams(next).toString()
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
    },
    [router, pathname],
  )
  const change = useCallback((patch: Partial<AssetFilters>) => navigate(updateFilters(latest.current, patch)), [navigate])
  const reset = useCallback(() => navigate(DEFAULT_FILTERS), [navigate])

  const list = useQuery({
    queryKey: ["assets", "list", query],
    queryFn: () => api<AssetListResponse>(`/api/assets${query ? `?${query}` : ""}`),
    placeholderData: (prev) => prev,
  })
  // Counts are tenant-wide (spec §6): they ignore every other filter.
  const summary = useQuery({
    queryKey: ["assets", "summary"],
    queryFn: () => api<AssetSummary>("/api/assets/summary"),
  })

  const onSort = (key: SortKey) =>
    change(
      filters.sort === key
        ? { dir: filters.dir === "asc" ? "desc" : "asc" }
        : { sort: key, dir: key === "created_at" ? "desc" : "asc" },
    )

  const dirty = filters.q !== "" || filters.category.length > 0 || filters.status.length > 0 || !!filters.location_id || !!filters.organization_id
  const items = list.data?.items ?? []
  const total = list.data?.total ?? 0
  const pageSize = list.data?.page_size || 20
  const pages = Math.max(1, Math.ceil(total / pageSize))

  // A bookmarked page past the end (e.g. after deleting the last row of a page) goes back in range.
  useEffect(() => {
    if (list.data && !list.isPlaceholderData && list.data.items.length === 0 && list.data.total > 0 && filters.page > 1) {
      change({ page: pages })
    }
  }, [list.data, list.isPlaceholderData, filters.page, pages, change])

  const addButton = (
    <Button asChild>
      <Link href="/assets/new">
        <Plus />
        {t("add")}
      </Link>
    </Button>
  )

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-[28px] sm:text-4xl">{t("title")}</h1>
        <div className="flex items-center gap-2">
          <div className="hidden items-center gap-2 md:flex">
            <ImportAction filters={filters} as="button" />
            <ExportAction filters={filters} as="button" />
          </div>
          <div className="md:hidden">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="secondary" size="icon" aria-label={t("moreActions")}>
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <ImportAction filters={filters} as="menu-item" />
                <ExportAction filters={filters} as="menu-item" />
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          {addButton}
        </div>
      </div>

      <SummaryCards
        summary={summary.data}
        active={cardFromStatuses(filters.status)}
        onChange={(status) => change({ status })}
      />

      <AssetFilterBar filters={filters} onChange={change} onReset={reset} dirty={dirty} />

      {list.isPending ? (
        <div className="flex flex-col gap-3" data-testid="assets-loading">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-16 rounded-lg" />
          ))}
        </div>
      ) : list.isError ? (
        <Card className="items-center gap-4 py-10 text-center">
          <p role="alert">{t("loadFailed")}</p>
          <Button variant="secondary" onClick={() => list.refetch()}>
            {tc("retry")}
          </Button>
        </Card>
      ) : items.length === 0 && !dirty && filters.page === 1 ? (
        <Card className="items-center gap-4 py-12 text-center">
          <p className="font-heading text-xl font-semibold">{t("empty.noneTitle")}</p>
          <p className="max-w-md text-muted-foreground">{t("empty.noneHint")}</p>
          <div className="flex flex-wrap justify-center gap-3">
            <ImportAction filters={filters} as="button" />
            {addButton}
          </div>
        </Card>
      ) : items.length === 0 ? (
        <Card className="items-center gap-4 py-12 text-center">
          <p className="font-heading text-xl font-semibold">{t("empty.filteredTitle")}</p>
          <p className="text-muted-foreground">{t("empty.filteredHint")}</p>
          <Button variant="secondary" onClick={reset}>
            {t("filters.reset")}
          </Button>
        </Card>
      ) : (
        <>
          <AssetTable items={items} sort={filters.sort} dir={filters.dir} onSort={onSort} onDelete={setDeleting} />
          <AssetCardList items={items} onDelete={setDeleting} />
          <div className="flex flex-col items-center justify-between gap-3 sm:flex-row">
            <p className="text-sm text-muted-foreground" aria-live="polite">
              {t("pagination.range", {
                from: format.number((filters.page - 1) * pageSize + 1),
                to: format.number((filters.page - 1) * pageSize + items.length),
                total,
              })}
            </p>
            {pages > 1 && (
              <nav aria-label={t("pagination.page", { page: filters.page, pages })} className="flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t("pagination.previous")}
                  disabled={filters.page <= 1}
                  onClick={() => change({ page: filters.page - 1 })}
                >
                  <ChevronLeft />
                </Button>
                <span className="px-2 text-sm sm:hidden">{t("pagination.page", { page: filters.page, pages })}</span>
                <div className="hidden items-center gap-1 sm:flex">
                  {pageWindow(filters.page, pages).map((p, i) =>
                    p === null ? (
                      <span key={`gap-${i}`} className="px-1 text-muted-foreground" aria-hidden>
                        …
                      </span>
                    ) : (
                      <Button
                        key={p}
                        variant={p === filters.page ? "secondary" : "ghost"}
                        size="icon"
                        aria-label={t("pagination.goTo", { page: p })}
                        aria-current={p === filters.page ? "page" : undefined}
                        onClick={() => change({ page: p })}
                      >
                        {p}
                      </Button>
                    ),
                  )}
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t("pagination.next")}
                  disabled={filters.page >= pages}
                  onClick={() => change({ page: filters.page + 1 })}
                >
                  <ChevronRight />
                </Button>
              </nav>
            )}
          </div>
        </>
      )}

      <DeleteAssetDialog asset={deleting} onClose={() => setDeleting(null)} />
    </div>
  )
}
