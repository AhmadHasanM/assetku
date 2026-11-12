"use client"

import { useTranslations } from "next-intl"
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

interface Props<T extends { active: boolean }> {
  /** Tenant or user to activate/deactivate; null closes the dialog. */
  target: T | null
  pending: boolean
  title: (target: T) => string
  description: (target: T) => string
  confirmLabel: (target: T) => string
  onCancel: () => void
  onConfirm: (target: T) => void
}

/** Confirmation before flipping a tenant's or user's active flag. Deactivating is destructive. */
export function ToggleActiveDialog<T extends { active: boolean }>({
  target,
  pending,
  title,
  description,
  confirmLabel,
  onCancel,
  onConfirm,
}: Props<T>) {
  const tc = useTranslations("common")
  return (
    <AlertDialog open={!!target} onOpenChange={(o) => !o && !pending && onCancel()}>
      <AlertDialogContent>
        {target && (
          <>
            <AlertDialogHeader>
              <AlertDialogTitle>{title(target)}</AlertDialogTitle>
              <AlertDialogDescription>{description(target)}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{tc("cancel")}</AlertDialogCancel>
              <AlertDialogAction
                variant={target.active ? "destructive" : "default"}
                disabled={pending}
                onClick={(e) => {
                  e.preventDefault()
                  onConfirm(target)
                }}
              >
                {confirmLabel(target)}
              </AlertDialogAction>
            </AlertDialogFooter>
          </>
        )}
      </AlertDialogContent>
    </AlertDialog>
  )
}
