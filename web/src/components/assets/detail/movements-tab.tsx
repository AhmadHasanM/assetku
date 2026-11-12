"use client"

import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { ArrowRight, Plus, Trash2 } from "lucide-react"
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
import { formatDate } from "@/lib/format"
import type { Locale, MasterRef } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { MovementDialog } from "./movement-dialog"

/** One row of GET /api/assets/{id}/movements. from_* is a snapshot; to_location_name is live. */
export interface Movement {
  id: string
  from_location_id: string | null
  from_location_name: string
  to_location_id: string
  to_location_name: string
  date: string
  moved_by: string
  reason: string
  created_at: string
}

/** Movement log (newest first), "Log Movement" and delete (which never reverts the location). */
export function MovementsTab({ asset }: { asset: { id: string; location: MasterRef | null } }) {
  const t = useTranslations("assetDetail.movements")
  const td = useTranslations("assetDetail")
  const tc = useTranslations("common")
  const locale = useLocale() as Locale
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const [adding, setAdding] = useState(false)
  const [deleting, setDeleting] = useState<Movement | null>(null)

  const list = useQuery({
    queryKey: ["assets", "detail", asset.id, "movements"],
    queryFn: () => api<{ items: Movement[] }>(`/api/assets/${asset.id}/movements`),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api<void>(`/api/movements/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["assets", "detail", asset.id, "movements"] })
      qc.invalidateQueries({ queryKey: ["assets", "detail", asset.id, "history"] })
      toast.success(t("deleted"))
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setDeleting(null),
  })

  const route = (m: Movement) => ({ from: m.from_location_name || "—", to: m.to_location_name || "—" })

  let body: React.ReactNode
  if (list.isPending) {
    body = (
      <div className="flex flex-col gap-3" aria-busy="true">
        {[0, 1].map((i) => (
          <Skeleton key={i} className="h-20 rounded-lg" />
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
  } else if (list.data.items.length === 0) {
    body = (
      <Card className="items-center px-6 text-center text-muted-foreground">
        <p>{t("empty")}</p>
      </Card>
    )
  } else {
    body = (
      <ul className="flex flex-col gap-3">
        {list.data.items.map((m) => (
          <li key={m.id}>
            <Card className="gap-2 p-4 sm:px-6">
              <div className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 flex-col gap-1">
                  <p className="flex flex-wrap items-center gap-x-2 font-heading text-lg font-semibold break-words">
                    <span className={m.from_location_name ? undefined : "text-muted-foreground"}>{route(m).from}</span>
                    <ArrowRight className="size-[18px] shrink-0 text-nebula-text" aria-label="→" />
                    <span>{route(m).to}</span>
                  </p>
                  <p className="text-sm text-muted-foreground">
                    {formatDate(m.date, locale)}
                    {m.moved_by && ` · ${t("by", { name: m.moved_by })}`}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t("deleteLabel", route(m))}
                  onClick={() => setDeleting(m)}
                >
                  <Trash2 />
                </Button>
              </div>
              {m.reason && <p className="text-sm break-words whitespace-pre-wrap">{m.reason}</p>}
            </Card>
          </li>
        ))}
      </ul>
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
      <MovementDialog asset={asset} open={adding} onOpenChange={setAdding} />
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && !remove.isPending && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{deleting ? t("deleteDescription", route(deleting)) : ""}</AlertDialogDescription>
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
