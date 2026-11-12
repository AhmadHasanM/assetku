"use client"

import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslations } from "next-intl"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { passwordProblem, type AdminUser } from "@/lib/admin"
import { applyFieldErrors } from "@/lib/form-errors"
import { useFieldErrorMessage } from "@/lib/use-error-message"

const schema = z
  .object({ password: z.string(), confirm: z.string() })
  .superRefine((v, ctx) => {
    const p = passwordProblem(v.password)
    if (p) ctx.addIssue({ code: "custom", path: ["password"], message: p })
    else if (v.password !== v.confirm) ctx.addIssue({ code: "custom", path: ["confirm"], message: "mismatch" })
  })
type Values = z.infer<typeof schema>

interface Props {
  user: AdminUser | null
  onOpenChange: (open: boolean) => void
  /** Throws the ApiError when the reset fails; field errors are shown inline. */
  onSubmit: (password: string) => Promise<void>
}

/** Sets a new password for a tenant user (their sessions are signed out by the API). */
export function ResetPasswordDialog({ user, onOpenChange, onSubmit }: Props) {
  const t = useTranslations("admin")
  const tc = useTranslations("common")
  const fieldMessage = useFieldErrorMessage()
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { password: "", confirm: "" } })

  useEffect(() => {
    if (user) reset({ password: "", confirm: "" })
  }, [user, reset])

  const submit = async (v: Values) => {
    try {
      await onSubmit(v.password)
    } catch (err) {
      applyFieldErrors(err, setError, ["password"])
    }
  }

  const message = (name: keyof Values) => {
    const e = errors[name]
    if (!e) return undefined
    const code = e.message || e.type
    return code === "mismatch" ? t("passwordMismatch") : fieldMessage(code)
  }

  return (
    <Dialog open={!!user} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{t("users.resetTitle")}</DialogTitle>
          <DialogDescription>{t("users.resetDescription", { name: user?.name ?? "" })}</DialogDescription>
        </DialogHeader>
        <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
          <FieldGroup className="gap-4">
            <Field data-invalid={!!errors.password}>
              <FieldLabel htmlFor="reset-password">{t("users.newPassword")}</FieldLabel>
              <Input
                id="reset-password"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.password}
                {...register("password")}
              />
              <FieldDescription>{t("passwordHint")}</FieldDescription>
              <FieldError>{message("password")}</FieldError>
            </Field>
            <Field data-invalid={!!errors.confirm}>
              <FieldLabel htmlFor="reset-confirm">{t("users.confirmPassword")}</FieldLabel>
              <Input
                id="reset-confirm"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.confirm}
                {...register("confirm")}
              />
              <FieldError>{message("confirm")}</FieldError>
            </Field>
          </FieldGroup>
          <DialogFooter className="-mx-0 -mb-0 border-0 bg-transparent p-0">
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {tc("cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? tc("saving") : t("users.resetSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
