"use client"

import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { api } from "@/lib/api"
import { applyFieldErrors } from "@/lib/form-errors"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"

// Error messages are field-error codes, translated at render time.
const schema = z
  .object({
    current_password: z.string().min(1, { error: "required" }),
    new_password: z.string().min(8, { error: "too_short" }),
    confirm_password: z.string(),
  })
  .refine((v) => v.new_password === v.confirm_password, {
    path: ["confirm_password"],
    error: "mismatch",
  })
type Values = z.infer<typeof schema>

/** Changes the password (POST /api/me/password). */
export function PasswordForm() {
  const t = useTranslations("profile")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const {
    register,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: "", new_password: "", confirm_password: "" },
  })

  const onSubmit = async ({ current_password, new_password }: Values) => {
    try {
      await api("/api/me/password", { method: "POST", json: { current_password, new_password } })
      reset()
      toast.success(t("passwordChanged"))
    } catch (err) {
      if (!applyFieldErrors(err, setError, ["current_password", "new_password"])) toast.error(errorMessage(err))
    }
  }

  const message = (name: keyof Values) => {
    const e = errors[name]
    if (!e) return undefined
    const code = e.message || e.type
    if (code === "mismatch") return t("passwordMismatch")
    if (name === "current_password" && code === "invalid") return t("currentPasswordInvalid")
    return fieldMessage(code)
  }

  return (
    <Card id="password" className="scroll-mt-24">
      <CardHeader>
        <CardTitle>
          <h2 className="text-[22px] font-semibold">{t("passwordTitle")}</h2>
        </CardTitle>
        <CardDescription>{t("passwordDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form noValidate onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-6">
          <FieldGroup className="gap-5">
            <Field data-invalid={!!errors.current_password}>
              <FieldLabel htmlFor="current_password">{t("currentPassword")}</FieldLabel>
              <Input
                id="current_password"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!errors.current_password}
                {...register("current_password")}
              />
              <FieldError>{message("current_password")}</FieldError>
            </Field>
            <Field data-invalid={!!errors.new_password}>
              <FieldLabel htmlFor="new_password">{t("newPassword")}</FieldLabel>
              <Input
                id="new_password"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.new_password}
                {...register("new_password")}
              />
              {errors.new_password ? (
                <FieldError>{message("new_password")}</FieldError>
              ) : (
                <FieldDescription>{t("passwordHint")}</FieldDescription>
              )}
            </Field>
            <Field data-invalid={!!errors.confirm_password}>
              <FieldLabel htmlFor="confirm_password">{t("confirmPassword")}</FieldLabel>
              <Input
                id="confirm_password"
                type="password"
                autoComplete="new-password"
                aria-invalid={!!errors.confirm_password}
                {...register("confirm_password")}
              />
              <FieldError>{message("confirm_password")}</FieldError>
            </Field>
          </FieldGroup>
          <div className="flex justify-end">
            <Button type="submit" disabled={isSubmitting} className="w-full sm:w-auto">
              {t("submitPassword")}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
