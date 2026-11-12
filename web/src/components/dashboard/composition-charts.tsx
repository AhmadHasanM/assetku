"use client"

import { Bar, BarChart, Cell, LabelList, XAxis, YAxis } from "recharts"
import { useFormatter, useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { ASSET_STATUS_TONE } from "@/lib/status"
import type { DashboardData } from "@/lib/types"

interface Row {
  key: string
  label: string
  count: number
  /** CSS color; theme-aware because every token flips with .dark. */
  fill: string
}

const BAR_PX = 20
const ROW_PX = 40

/**
 * Horizontal bars, one per group, labelled on the axis and at the tip, so
 * identity and value never depend on color. A hidden table carries the same numbers.
 */
function CompositionBars({ title, rows, config }: { title: string; rows: Row[]; config: ChartConfig }) {
  const t = useTranslations("dashboard.chart")
  const format = useFormatter()
  const total = rows.reduce((n, r) => n + r.count, 0)
  const height = rows.length * ROW_PX + 16

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-heading text-xl">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {total === 0 ? (
          <p className="py-6 text-center text-muted-foreground">{t("empty")}</p>
        ) : (
          <>
            <ChartContainer config={config} className="aspect-auto w-full text-sm" style={{ height }} aria-hidden>
              <BarChart data={rows} layout="vertical" margin={{ top: 8, right: 40, bottom: 8, left: 0 }}>
                <XAxis type="number" dataKey="count" hide allowDecimals={false} />
                <YAxis
                  type="category"
                  dataKey="label"
                  width={128}
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  tick={{ fontSize: 14, fill: "var(--muted-foreground)" }}
                />
                <ChartTooltip
                  cursor={{ fill: "var(--nebula)", fillOpacity: 0.1 }}
                  content={
                    <ChartTooltipContent
                      hideLabel
                      nameKey="key"
                      className="min-w-40 text-sm"
                      formatter={(value, _name, item) => {
                        const row = item.payload as Row
                        return (
                          <>
                            <span aria-hidden className="size-2.5 shrink-0 rounded-[2px]" style={{ background: row.fill }} />
                            <span className="flex flex-1 items-center justify-between gap-4 leading-none">
                              <span className="text-muted-foreground">{row.label}</span>
                              <span className="font-medium text-foreground tabular-nums">{format.number(Number(value))}</span>
                            </span>
                          </>
                        )
                      }}
                    />
                  }
                />
                <Bar dataKey="count" barSize={BAR_PX} radius={[0, 4, 4, 0]} isAnimationActive={false}>
                  {rows.map((r) => (
                    <Cell key={r.key} fill={r.fill} />
                  ))}
                  <LabelList
                    dataKey="count"
                    position="right"
                    offset={8}
                    className="fill-foreground"
                    fontSize={14}
                    formatter={(v) => format.number(Number(v))}
                  />
                </Bar>
              </BarChart>
            </ChartContainer>
            <table className="sr-only">
              <caption>{title}</caption>
              <thead>
                <tr>
                  <th scope="col">{t("dimension")}</th>
                  <th scope="col">{t("count")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.key}>
                    <th scope="row">{r.label}</th>
                    <td>{format.number(r.count)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </CardContent>
    </Card>
  )
}

/** Asset counts per category (one series, one hue) and per status (status colors, spec §3). */
export function CompositionCharts({
  byCategory,
  byStatus,
}: {
  byCategory: DashboardData["by_category"]
  byStatus: DashboardData["by_status"]
}) {
  const t = useTranslations("dashboard")
  const tCat = useTranslations("category")
  const tStatus = useTranslations("status")

  const categoryRows: Row[] = byCategory.map((c) => ({
    key: c.key,
    label: tCat(c.key),
    count: c.count,
    fill: "var(--chart-1)",
  }))
  const statusRows: Row[] = byStatus.map((s) => ({
    key: s.key,
    label: tStatus(s.key),
    count: s.count,
    fill: `var(--${ASSET_STATUS_TONE[s.key] ?? "locked"})`,
  }))
  const categoryConfig = Object.fromEntries(
    categoryRows.map((r) => [r.key, { label: r.label, color: r.fill }]),
  ) satisfies ChartConfig
  const statusConfig = Object.fromEntries(
    statusRows.map((r) => [r.key, { label: r.label, color: r.fill }]),
  ) satisfies ChartConfig

  return (
    <>
      <CompositionBars title={t("byCategory.title")} rows={categoryRows} config={categoryConfig} />
      <CompositionBars title={t("byStatus.title")} rows={statusRows} config={statusConfig} />
    </>
  )
}
