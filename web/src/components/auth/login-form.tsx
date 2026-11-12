"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslations } from "next-intl"
import { useTheme } from "next-themes"
import { useQueryClient } from "@tanstack/react-query"
import { Loader2, Star } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { api } from "@/lib/api"
import { ME_QUERY_KEY } from "@/lib/me"
import { writeLocaleCookie } from "@/lib/preferences"
import { safeNext } from "@/lib/safe-next"
import type { MeResponse } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"

export function LoginForm({ next }: { next: string | null }) {
  const t = useTranslations("auth")
  const errorMessage = useErrorMessage()
  const router = useRouter()
  const qc = useQueryClient()
  const { setTheme } = useTheme()
  const [formError, setFormError] = useState<string | null>(null)

  const schema = z.object({
    email: z.string().trim().min(1, t("emailRequired")).pipe(z.email(t("emailInvalid"))),
    password: z.string().min(1, t("passwordRequired")),
  })
  type Values = z.infer<typeof schema>

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { email: "", password: "" } })

  const onSubmit = async (values: Values) => {
    setFormError(null)
    try {
      const { user } = await api<MeResponse>("/api/auth/login", { method: "POST", json: values })
      qc.setQueryData(ME_QUERY_KEY, user)
      writeLocaleCookie(user.locale)
      setTheme(user.theme)
      router.replace(safeNext(next))
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-6">
      <FieldGroup className="gap-5">
        <Field data-invalid={!!errors.email}>
          <FieldLabel htmlFor="email">{t("email")}</FieldLabel>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            inputMode="email"
            autoFocus
            placeholder={t("emailPlaceholder")}
            aria-invalid={!!errors.email}
            {...register("email")}
          />
          <FieldError errors={[errors.email]} />
        </Field>
        <Field data-invalid={!!errors.password}>
          <FieldLabel htmlFor="password">{t("password")}</FieldLabel>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            aria-invalid={!!errors.password}
            {...register("password")}
          />
          <FieldError errors={[errors.password]} />
        </Field>
      </FieldGroup>
      {formError && (
        <p role="alert" className="rounded-md bg-error/15 px-4 py-3 text-sm font-medium text-error">
          {formError}
        </p>
      )}
      <Button type="submit" size="lg" disabled={isSubmitting} className="w-full">
        {isSubmitting ? <Loader2 className="animate-spin" /> : <Star className="fill-current" />}
        {isSubmitting ? t("submitting") : t("submit")}
      </Button>
    </form>
  )
}
