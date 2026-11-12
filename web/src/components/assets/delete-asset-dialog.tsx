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
  asset: { id: string; name: string } | null
  onClose: () => void
}

/** Permanent-delete confirmation. On success it refreshes the asset list and summary. */
export function DeleteAssetDialog({ asset, onClose }: Props) {
  const t = useTranslations("assets.deleteDialog")
  const tc = useTranslations("common")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()

  const remove = useMutation({
    mutationFn: (id: string) => api<void>(`/api/assets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      // Prefix match covers both the list and the summary query.
      qc.invalidateQueries({ queryKey: ["assets"] })
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
          <AlertDialogDescription>{t("description", { name: asset?.name ?? "" })}</AlertDialogDescription>
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
