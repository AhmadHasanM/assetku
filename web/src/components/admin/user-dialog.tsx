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

// Error messages are field-error codes, translated at render time.
const schema = z.object({
  name: z.string().trim().min(1, { error: "required" }),
  email: z.string().trim().min(1, { error: "required" }).pipe(z.email({ error: "invalid" })),
  position: z.string(),
  password: z.string(),
})
export type UserValues = z.infer<typeof schema>

const EMPTY: UserValues = { name: "", email: "", position: "", password: "" }

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** User being edited, or null when adding. */
  user: AdminUser | null
  /** Throws the ApiError when the save fails; field errors are shown inline. */
  onSubmit: (values: Partial<UserValues>) => Promise<void>
}

/** Add (name, email, position, initial password) or edit (name, position) a tenant user. */
export function UserDialog({ open, onOpenChange, user, onSubmit }: Props) {
  const t = useTranslations("admin")
  const tc = useTranslations("common")
  const fieldMessage = useFieldErrorMessage()
  const editing = !!user
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<UserValues>({
    resolver: zodResolver(
      schema.superRefine((v, ctx) => {
        if (editing) return
        const p = passwordProblem(v.password)
        if (p) ctx.addIssue({ code: "custom", path: ["password"], message: p })
      }),
    ),
    defaultValues: EMPTY,
  })

  useEffect(() => {
    if (!open) return
    reset(user ? { name: user.name, email: user.email, position: user.position, password: "" } : EMPTY)
  }, [open, user, reset])

  const submit = async (v: UserValues) => {
    const body: Partial<UserValues> = editing
      ? { name: v.name.trim(), position: v.position.trim() }
      : { name: v.name.trim(), email: v.email.trim(), position: v.position.trim(), password: v.password }
    try {
      await onSubmit(body)
    } catch (err) {
      // Anything not shown inline was already reported by the caller.
      applyFieldErrors(err, setError, ["name", "email", "position", "password"])
    }
  }

  const message = (name: keyof UserValues) => {
    const e = errors[name]
    return e ? fieldMessage(e.message || e.type) : undefined
  }

  const title = editing ? t("users.editTitle") : t("users.addTitle")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{title}</DialogTitle>
          <DialogDescription className="sr-only">{title}</DialogDescription>
        </DialogHeader>
        <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
          <FieldGroup className="gap-4">
            <Field data-invalid={!!errors.name}>
              <FieldLabel htmlFor="user-name">{t("users.name")}</FieldLabel>
              <Input id="user-name" autoComplete="off" aria-invalid={!!errors.name} {...register("name")} />
              <FieldError>{message("name")}</FieldError>
            </Field>
            <Field data-invalid={!!errors.email}>
              <FieldLabel htmlFor="user-email">{t("users.email")}</FieldLabel>
              <Input
                id="user-email"
                type="email"
                inputMode="email"
                autoComplete="off"
                readOnly={editing}
                className={editing ? "text-muted-foreground" : undefined}
                aria-invalid={!!errors.email}
                {...register("email")}
              />
              {editing && <FieldDescription>{t("users.emailLocked")}</FieldDescription>}
              <FieldError>{message("email")}</FieldError>
            </Field>
            <Field data-invalid={!!errors.position}>
              <FieldLabel htmlFor="user-position">
                {t("users.position")}
                <span className="ml-1 font-normal text-subtle-foreground">({tc("optional")})</span>
              </FieldLabel>
              <Input id="user-position" autoComplete="off" aria-invalid={!!errors.position} {...register("position")} />
              <FieldError>{message("position")}</FieldError>
            </Field>
            {!editing && (
              <Field data-invalid={!!errors.password}>
                <FieldLabel htmlFor="user-password">{t("users.initialPassword")}</FieldLabel>
                <Input
                  id="user-password"
                  type="password"
                  autoComplete="new-password"
                  aria-invalid={!!errors.password}
                  {...register("password")}
                />
                <FieldDescription>{t("passwordHint")}</FieldDescription>
                <FieldError>{message("password")}</FieldError>
              </Field>
            )}
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
