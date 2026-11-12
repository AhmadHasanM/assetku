"use client"

import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
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
import { api } from "@/lib/api"
import { useErrorMessage } from "@/lib/use-error-message"

interface Props {
  asset: { id: string; name: string; asset_tag: string } | null
  onClose: () => void
  /** Called before the caches refresh, e.g. to leave the deleted asset's page. */
  onDeleted?: (id: string) => void
}

/** Permanent-delete confirmation naming the asset and its tag. On success it refreshes the asset list and summary. */
export function DeleteAssetDialog({ asset, onClose, onDeleted }: Props) {
  const t = useTranslations("assets.deleteDialog")
  const tc = useTranslations("common")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()

  const remove = useMutation({
    mutationFn: (id: string) => api<void>(`/api/assets/${id}`, { method: "DELETE" }),
    onSuccess: (_, id) => {
      onDeleted?.(id)
      // The deleted asset's own queries (detail, tabs) could only refetch into 404s.
      qc.removeQueries({ queryKey: ["assets", "detail", id] })
      // Prefix match covers both the list and the summary query.
      qc.invalidateQueries({ queryKey: ["assets"] })
      qc.invalidateQueries({ queryKey: ["dashboard"] })
      toast.success(t("done"))
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: onClose,
  })

  return (
    <AlertDialog open={!!asset} onOpenChange={(o) => !o && !remove.isPending && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("title")}</AlertDialogTitle>
          <AlertDialogDescription>{t("description", { name: asset?.name ?? "", tag: asset?.asset_tag ?? "" })}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{tc("cancel")}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(e) => {
              e.preventDefault()
              if (asset) remove.mutate(asset.id)
            }}
          >
            {remove.isPending ? t("deleting") : t("confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
