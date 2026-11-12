"use client"

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslations } from "next-intl"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { AdminTenant } from "@/lib/admin"
import { applyFieldErrors } from "@/lib/form-errors"
import { useFieldErrorMessage } from "@/lib/use-error-message"

const schema = z.object({ name: z.string().trim().min(1, { error: "required" }) })
type Values = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Tenant being renamed, or null when adding. */
  tenant: AdminTenant | null
  /** Throws the ApiError when the save fails; field errors are shown inline. */
  onSubmit: (values: Values) => Promise<void>
}

/** Add a tenant or rename one. */
export function TenantDialog({ open, onOpenChange, tenant, onSubmit }: Props) {
  const t = useTranslations("admin")
  const tc = useTranslations("common")
  const fieldMessage = useFieldErrorMessage()
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { name: "" } })

  useEffect(() => {
    if (open) reset({ name: tenant?.name ?? "" })
  }, [open, tenant, reset])

  const submit = async (v: Values) => {
    try {
      await onSubmit({ name: v.name.trim() })
    } catch (err) {
      applyFieldErrors(err, setError, ["name"])
    }
  }

  const title = tenant ? t("tenants.renameTitle") : t("tenants.addTitle")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{title}</DialogTitle>
          <DialogDescription className="sr-only">{title}</DialogDescription>
        </DialogHeader>
        <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
          <FieldGroup>
            <Field data-invalid={!!errors.name}>
              <FieldLabel htmlFor="tenant-name">{t("tenants.name")}</FieldLabel>
              <Input id="tenant-name" autoComplete="organization" aria-invalid={!!errors.name} {...register("name")} />
              <FieldError>{errors.name ? fieldMessage(errors.name.message || errors.name.type) : undefined}</FieldError>
            </Field>
          </FieldGroup>
          <DialogFooter className="-mx-0 -mb-0 border-0 bg-transparent p-0">
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {tc("cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? tc("saving") : tc("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
