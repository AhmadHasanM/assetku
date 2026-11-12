"use client"

import Link from "next/link"
import { CalendarClock, ChevronRight } from "lucide-react"
import { useFormatter, useLocale, useTranslations } from "next-intl"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { daysBetween, formatDate, todayJakarta } from "@/lib/format"
import { DASHBOARD_LIST_CAP, type DashboardData, type Locale } from "@/lib/types"

const SHOWN = 8
/** Same window as the default reminder (H-7). */
const DUE_SOON_DAYS = 7

/** "Maintenance Mendatang": next maintenance within 30 days or overdue, soonest first. */
export function UpcomingMaintenance({ items }: { items: DashboardData["upcoming_maintenance"] }) {
  const t = useTranslations("dashboard.upcoming")
  const locale = useLocale() as Locale
  const format = useFormatter()
  const rest = items.length - SHOWN
  // A full page from the API means there may be more rows than we got.
  const restText = format.number(rest) + (items.length >= DASHBOARD_LIST_CAP ? "+" : "")
  const today = todayJakarta()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-heading text-xl">{t("title")}</CardTitle>
        <CardDescription>{t("hint")}</CardDescription>
      </CardHeader>
      <CardContent className="px-3 sm:px-4">
        {items.length === 0 ? (
          <p className="px-2 py-6 text-center text-muted-foreground">{t("empty")}</p>
        ) : (
          <>
            <ul className="flex flex-col">
              {items.slice(0, SHOWN).map((m) => {
                const dueSoon = !m.overdue && daysBetween(today, m.next_date) <= DUE_SOON_DAYS
                return (
                  <li key={m.asset_id} className="border-b last:border-b-0">
                    <Link
                      href={`/assets/${m.asset_id}`}
                      className="flex min-h-11 items-center gap-3 rounded-md px-2 py-3 outline-none transition-colors hover:bg-nebula/10 focus-visible:ring-3 focus-visible:ring-ring/50"
                    >
                      <span className="flex min-w-0 flex-1 flex-col gap-1">
                        <span className="font-mono text-sm text-nebula-text">{m.asset_tag}</span>
                        <span className="truncate font-semibold">{m.name}</span>
                        <span className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                          <span className="inline-flex items-center gap-1">
                            <CalendarClock aria-hidden className="size-3.5" />
                            <time dateTime={m.next_date}>{formatDate(m.next_date, locale)}</time>
                          </span>
                          {m.overdue ? (
                            <Badge variant="error">{t("overdue")}</Badge>
                          ) : dueSoon ? (
                            <Badge variant="warning">{t("dueSoon")}</Badge>
                          ) : null}
                        </span>
                      </span>
                      <ChevronRight aria-hidden className="size-4 shrink-0 text-subtle-foreground" />
                    </Link>
                  </li>
                )
              })}
            </ul>
            {rest > 0 && (
              <p className="px-2 pt-3 text-sm text-muted-foreground">{t("more", { count: restText })}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
