"use client"

import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useUpdateMe } from "@/lib/me"
import { applyFieldErrors } from "@/lib/form-errors"
import type { Me } from "@/lib/types"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"

const schema = z.object({
  name: z.string().trim().min(1, { error: "required" }),
  position: z.string().trim(),
})
type Values = z.infer<typeof schema>

/** Edits name and position (PATCH /api/me). */
export function ProfileForm({ me }: { me: Me }) {
  const t = useTranslations("profile")
  const tc = useTranslations("common")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const updateMe = useUpdateMe()
  const {
    register,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    values: { name: me.name, position: me.position },
  })

  const onSubmit = async (values: Values) => {
    try {
      const saved = await updateMe.mutateAsync(values)
      reset({ name: saved.name, position: saved.position })
      toast.success(t("saved"))
    } catch (err) {
      if (!applyFieldErrors(err, setError, ["name", "position"])) toast.error(errorMessage(err))
    }
  }

  const nameError = errors.name && fieldMessage(errors.name.message || errors.name.type)

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2 className="text-[22px] font-semibold">{t("detailsTitle")}</h2>
        </CardTitle>
        <CardDescription>{t("detailsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form noValidate onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-6">
          <FieldGroup className="gap-5">
            <Field data-invalid={!!errors.name}>
              <FieldLabel htmlFor="name">{t("name")}</FieldLabel>
              <Input id="name" autoComplete="name" aria-invalid={!!errors.name} {...register("name")} />
              <FieldError>{nameError}</FieldError>
            </Field>
            <Field>
              <FieldLabel htmlFor="email">{t("email")}</FieldLabel>
              <Input id="email" value={me.email} readOnly disabled />
            </Field>
            <Field>
              <FieldLabel htmlFor="position">{t("position")}</FieldLabel>
              <Input id="position" autoComplete="organization-title" {...register("position")} />
            </Field>
          </FieldGroup>
          <div className="flex justify-end">
            <Button type="submit" disabled={isSubmitting || !isDirty} className="w-full sm:w-auto">
              {isSubmitting ? tc("saving") : tc("save")}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
