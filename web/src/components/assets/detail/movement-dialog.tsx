"use client"

import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { api, ApiError } from "@/lib/api"
import { todayJakarta } from "@/lib/format"
import { useMe } from "@/lib/me"
import type { MasterRef } from "@/lib/types"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"
import { isIsoDate } from "../asset-schema"
import { DateInput } from "../date-input"
import { MasterCombobox } from "../master-combobox"

const schema = z.object({
  to_location: z.object({ id: z.string(), name: z.string() }).nullable().refine((v) => Boolean(v), { error: "required" }),
  date: z.string().refine(isIsoDate, { error: "required" }),
  moved_by: z.string(),
  reason: z.string(),
})
type Values = z.infer<typeof schema>
type FieldName = keyof Values
const API_FIELD: Record<string, FieldName> = {
  to_location_id: "to_location",
  date: "date",
  moved_by: "moved_by",
  reason: "reason",
}

interface Props {
  asset: { id: string; location: MasterRef | null }
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** "Log Movement": moves the asset to another location (the API updates the asset's location). */
export function MovementDialog({ asset, open, onOpenChange }: Props) {
  const t = useTranslations("assetDetail.movements")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{t("dialogTitle")}</DialogTitle>
          <DialogDescription>{t("dialogDescription")}</DialogDescription>
        </DialogHeader>
        <MovementForm asset={asset} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

function MovementForm({ asset, onDone }: { asset: Props["asset"]; onDone: () => void }) {
  const t = useTranslations("assetDetail.movements")
  const tc = useTranslations("common")
  const tForm = useTranslations("assetForm")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const qc = useQueryClient()
  const me = useMe()
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { to_location: null, date: todayJakarta(), moved_by: me.data?.name ?? "", reason: "" },
  })

  const submit = async (v: Values) => {
    if (asset.location && v.to_location?.id === asset.location.id) {
      setError("to_location", { type: "same_as_current" }, { shouldFocus: true })
      return
    }
    try {
      await api(`/api/assets/${asset.id}/movements`, {
        method: "POST",
        json: { to_location_id: v.to_location?.id, date: v.date, moved_by: v.moved_by.trim(), reason: v.reason.trim() },
      })
      // The header shows the new location; list, summary, history and movements all live under ["assets"].
      qc.invalidateQueries({ queryKey: ["assets"] })
      qc.invalidateQueries({ queryKey: ["dashboard"] })
      toast.success(t("saved"))
      onDone()
    } catch (err) {
      let shown = false
      if (err instanceof ApiError) {
        for (const [apiField, code] of Object.entries(err.fields)) {
          const f = API_FIELD[apiField]
          if (!f) continue
          setError(f, { type: code }, { shouldFocus: !shown })
          shown = true
        }
      }
      if (!shown) toast.error(errorMessage(err))
    }
  }

  const id = (f: FieldName) => `movement-${f}`
  const errorText = (f: FieldName) => {
    const e = errors[f]
    return e ? fieldMessage(e.message || e.type) : undefined
  }
  const label = (f: FieldName, apiName: string, required = false) => (
    <FieldLabel htmlFor={id(f)} className={cn(required && "after:-ml-1 after:text-error after:content-['*']")}>
      {t(`fields.${apiName}`)}
    </FieldLabel>
  )

  return (
    <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
      <p className="text-sm text-muted-foreground">
        {t("currentLocation", { name: asset.location?.name ?? "—" })}
      </p>
      <Field data-invalid={!!errors.to_location}>
        {label("to_location", "to_location_id", true)}
        <Controller
          control={control}
          name="to_location"
          render={({ field: c }) => (
            <MasterCombobox
              kind="locations"
              id={id("to_location")}
              invalid={!!errors.to_location}
              value={c.value}
              onChange={c.onChange}
              onBlur={c.onBlur}
              placeholder={tForm("placeholders.master")}
            />
          )}
        />
        <FieldError>{errorText("to_location")}</FieldError>
      </Field>
      <Field data-invalid={!!errors.date}>
        {label("date", "date", true)}
        <Controller
          control={control}
          name="date"
          render={({ field: c }) => (
            <DateInput id={id("date")} invalid={!!errors.date} value={c.value} onChange={c.onChange} onBlur={c.onBlur} clearable={false} />
          )}
        />
        <FieldError>{errorText("date")}</FieldError>
      </Field>
      <Field data-invalid={!!errors.moved_by}>
        {label("moved_by", "moved_by")}
        <Input id={id("moved_by")} autoComplete="off" {...register("moved_by")} />
        <FieldError>{errorText("moved_by")}</FieldError>
      </Field>
      <Field data-invalid={!!errors.reason}>
        {label("reason", "reason")}
        <Textarea id={id("reason")} rows={3} {...register("reason")} />
        <FieldError>{errorText("reason")}</FieldError>
      </Field>
      <DialogFooter className="-mx-0 -mb-0 border-0 bg-transparent p-0">
        <Button type="button" variant="secondary" onClick={onDone}>
          {tc("cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? tc("saving") : tc("save")}
        </Button>
      </DialogFooter>
    </form>
  )
}
