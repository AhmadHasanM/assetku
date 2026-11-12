"use client"

import { useRouter } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { SummaryCards } from "@/components/assets/summary-cards"
import { AttentionList } from "@/components/dashboard/attention-list"
import { CompositionCharts } from "@/components/dashboard/composition-charts"
import { UpcomingMaintenance } from "@/components/dashboard/upcoming-maintenance"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { DEFAULT_FILTERS, toSearchParams } from "@/lib/filters"
import type { DashboardData } from "@/lib/types"

export default function DashboardPage() {
  const t = useTranslations("dashboard")
  const tc = useTranslations("common")
  const router = useRouter()
  const dashboard = useQuery({
    queryKey: ["dashboard"],
    queryFn: () => api<DashboardData>("/api/dashboard"),
  })
  const d = dashboard.data

  const openAssets = (status: string[]) => {
    const qs = toSearchParams({ ...DEFAULT_FILTERS, status }).toString()
    router.push(qs ? `/assets?${qs}` : "/assets")
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <h1 className="text-[28px] sm:text-4xl">{t("title")}</h1>

      <SummaryCards summary={d?.summary} active={null} onChange={openAssets} />

      {dashboard.isError ? (
        <Card className="items-center gap-4 py-10 text-center">
          <p role="alert">{t("loadFailed")}</p>
          <Button variant="secondary" onClick={() => dashboard.refetch()}>
            {tc("retry")}
          </Button>
        </Card>
      ) : !d ? (
        <div className="grid gap-4 lg:grid-cols-2 lg:gap-6" data-testid="dashboard-loading">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-72 rounded-lg" />
          ))}
        </div>
      ) : (
        <div className="grid items-start gap-4 lg:grid-cols-2 lg:gap-6">
          <AttentionList items={d.attention} />
          <UpcomingMaintenance items={d.upcoming_maintenance} />
          <CompositionCharts byCategory={d.by_category} byStatus={d.by_status} />
        </div>
      )}
    </div>
  )
}
