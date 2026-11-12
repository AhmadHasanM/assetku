"use client"

import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { CalendarClock, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { formatDate, formatRupiah } from "@/lib/format"
import type { Locale, MaintenanceType } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { MaintenanceDialog } from "./maintenance-dialog"

/** One row of GET /api/assets/{id}/maintenance. */
export interface MaintenanceRecord {
  id: string
  date: string
  type: MaintenanceType
  performed_by: string
  cost: string | null
  next_date: string | null
  reminder_days: number | null
  notes: string
  created_by_name: string
  created_at: string
}

interface MaintenanceList {
  items: MaintenanceRecord[]
  /** The latest record's next_date (latest-maintenance rule), if it has one. */
  next_due: { date: string; record_id: string } | null
}

/** Next due card, the record list (newest first) and add/delete. */
export function MaintenanceTab({ assetId }: { assetId: string }) {
  const t = useTranslations("assetDetail.maintenance")
  const td = useTranslations("assetDetail")
  const tc = useTranslations("common")
  const tType = useTranslations("maintenanceType")
  const locale = useLocale() as Locale
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [deleting, setDeleting] = useState<MaintenanceRecord | null>(null)

  const list = useQuery({
    queryKey: ["assets", "detail", assetId, "maintenance"],
    queryFn: () => api<MaintenanceList>(`/api/assets/${assetId}/maintenance`),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api<void>(`/api/maintenance/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["assets", "detail", assetId, "maintenance"] })
      qc.invalidateQueries({ queryKey: ["assets", "detail", assetId, "history"] })
      toast.success(t("deleted"))
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setDeleting(null),
  })

  const typeDate = (r: MaintenanceRecord) => ({ type: tType(r.type), date: formatDate(r.date, locale) })

  let body: React.ReactNode
  if (list.isPending) {
    body = (
      <div className="flex flex-col gap-3" aria-busy="true">
        {[0, 1].map((i) => (
          <Skeleton key={i} className="h-24 rounded-lg" />
        ))}
      </div>
    )
  } else if (list.isError) {
    body = (
      <Card className="items-center gap-4 px-6 text-center">
        <p role="alert">{t("loadFailed")}</p>
        <Button variant="secondary" onClick={() => list.refetch()}>
          {tc("retry")}
        </Button>
      </Card>
    )
  } else {
    const { items, next_due } = list.data
    const dueRecord = next_due ? items.find((r) => r.id === next_due.record_id) : undefined
    body = (
      <div className="flex flex-col gap-4">
        {next_due && (
          <Card className="flex-row items-center gap-4 border-nebula-text/40 px-4 sm:px-6">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-nebula/15 text-nebula-text">
              <CalendarClock aria-hidden />
            </span>
            <div className="flex min-w-0 flex-col">
              <p className="text-sm text-muted-foreground">{t("next")}</p>
              <p className="font-heading text-xl font-semibold">{formatDate(next_due.date, locale)}</p>
              {dueRecord && <p className="text-sm text-muted-foreground">{t("nextHint", typeDate(dueRecord))}</p>}
            </div>
          </Card>
        )}
        {items.length === 0 ? (
          <Card className="items-center px-6 text-center text-muted-foreground">
            <p>{t("empty")}</p>
          </Card>
        ) : (
          <ul className="flex flex-col gap-3">
            {items.map((r) => (
              <li key={r.id}>
                <Card className="gap-2 p-4 sm:px-6">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 flex-col">
                      <p className="font-heading text-lg font-semibold">{tType(r.type)}</p>
                      <p className="text-sm text-muted-foreground">{formatDate(r.date, locale)}</p>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t("deleteLabel", typeDate(r))}
                      onClick={() => setDeleting(r)}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                  <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
                    <div>
                      <dt className="text-tiny text-subtle-foreground">{t("fields.performed_by")}</dt>
                      <dd className="break-words">{r.performed_by || "—"}</dd>
                    </div>
                    <div>
                      <dt className="text-tiny text-subtle-foreground">{t("fields.cost")}</dt>
                      <dd>{formatRupiah(r.cost)}</dd>
                    </div>
                    <div>
                      <dt className="text-tiny text-subtle-foreground">{t("fields.next_date")}</dt>
                      <dd>
                        {formatDate(r.next_date, locale)}
                        {r.next_date && r.reminder_days !== null && (
                          <span className="block text-muted-foreground">
                            {t("reminderShort", { days: r.reminder_days })}
                          </span>
                        )}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-tiny text-subtle-foreground">{t("fields.notes")}</dt>
                      <dd className="break-words whitespace-pre-wrap">{r.notes || "—"}</dd>
                    </div>
                  </dl>
                  {r.created_by_name && (
                    <p className="text-sm text-muted-foreground">{t("recordedBy", { name: r.created_by_name })}</p>
                  )}
                </Card>
              </li>
            ))}
          </ul>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button variant="secondary" onClick={() => setAdding(true)}>
          <Plus />
          {t("add")}
        </Button>
      </div>
      {body}
      <MaintenanceDialog assetId={assetId} open={adding} onOpenChange={setAdding} />
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && !remove.isPending && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{deleting ? t("deleteDescription", typeDate(deleting)) : ""}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={remove.isPending}
              onClick={(e) => {
                e.preventDefault()
                if (deleting) remove.mutate(deleting.id)
              }}
            >
              {remove.isPending ? td("deleting") : td("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
