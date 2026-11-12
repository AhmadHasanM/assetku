"use client"

import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { api } from "@/lib/api"
import { applyFieldErrors } from "@/lib/form-errors"
import { todayJakarta } from "@/lib/format"
import { MAINTENANCE_TYPES } from "@/lib/types"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"
import { isIsoDate, parseCost } from "../asset-schema"
import { DateInput } from "../date-input"

const DEFAULT_REMINDER_DAYS = "7"
const MAX_REMINDER_DAYS = 3650

// Mirrors maintenance.validate in the API.
const schema = z
  .object({
    date: z.string().refine(isIsoDate, { error: "required" }),
    type: z
      .string()
      .refine((v) => v !== "", { error: "required" })
      .refine((v) => v === "" || (MAINTENANCE_TYPES as readonly string[]).includes(v), { error: "invalid" }),
    performed_by: z.string(),
    cost: z.string().refine((v) => v.trim() === "" || parseCost(v) !== null, { error: "invalid" }),
    next_date: z.string().refine((v) => v === "" || isIsoDate(v), { error: "invalid" }),
    reminder_days: z.string(),
    notes: z.string(),
  })
  .superRefine((v, ctx) => {
    if (v.next_date && isIsoDate(v.date) && v.next_date < v.date) {
      ctx.addIssue({ code: "custom", path: ["next_date"], message: "before_date" })
    }
    if (v.next_date) {
      const s = v.reminder_days.trim()
      if (s !== "" && !(/^\d+$/.test(s) && Number(s) <= MAX_REMINDER_DAYS)) {
        ctx.addIssue({ code: "custom", path: ["reminder_days"], message: "invalid" })
      }
    }
  })
type Values = z.infer<typeof schema>
type FieldName = keyof Values
const FIELDS: FieldName[] = ["date", "type", "performed_by", "cost", "next_date", "reminder_days", "notes"]

interface Props {
  assetId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** "Add Record": a maintenance record (records cannot be edited, only deleted). */
export function MaintenanceDialog({ assetId, open, onOpenChange }: Props) {
  const t = useTranslations("assetDetail.maintenance")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{t("dialogTitle")}</DialogTitle>
          <DialogDescription className="sr-only">{t("dialogTitle")}</DialogDescription>
        </DialogHeader>
        {/* Remounts on every open, so the form starts empty with today's date. */}
        <MaintenanceForm assetId={assetId} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

function MaintenanceForm({ assetId, onDone }: { assetId: string; onDone: () => void }) {
  const t = useTranslations("assetDetail.maintenance")
  const tc = useTranslations("common")
  const tType = useTranslations("maintenanceType")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const qc = useQueryClient()
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      date: todayJakarta(),
      type: "",
      performed_by: "",
      cost: "",
      next_date: "",
      reminder_days: DEFAULT_REMINDER_DAYS,
      notes: "",
    },
  })
  const nextDate = useWatch({ control, name: "next_date" })
  const reminderDays = useWatch({ control, name: "reminder_days" })

  const submit = async (v: Values) => {
    const reminder = v.reminder_days.trim()
    try {
      await api(`/api/assets/${assetId}/maintenance`, {
        method: "POST",
        json: {
          date: v.date,
          type: v.type,
          performed_by: v.performed_by.trim(),
          cost: v.cost.trim() ? parseCost(v.cost) : null,
          next_date: v.next_date || null,
          // Blank means the API default (7) when a next date is set.
          reminder_days: v.next_date && reminder !== "" ? Number(reminder) : null,
          notes: v.notes,
        },
      })
      qc.invalidateQueries({ queryKey: ["assets", "detail", assetId, "maintenance"] })
      qc.invalidateQueries({ queryKey: ["assets", "detail", assetId, "history"] })
      toast.success(t("saved"))
      onDone()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) toast.error(errorMessage(err))
    }
  }

  const id = (f: FieldName) => `maintenance-${f}`
  const errorText = (f: FieldName) => {
    const e = errors[f]
    return e ? fieldMessage(e.message || e.type) : undefined
  }
  const field = (
    f: FieldName,
    input: React.ReactNode,
    opts: { required?: boolean; hint?: string; wide?: boolean } = {},
  ) => (
    <Field data-invalid={!!errors[f]} className={cn(opts.wide && "sm:col-span-2")}>
      <FieldLabel
        htmlFor={id(f)}
        className={cn(opts.required && "after:-ml-1 after:text-error after:content-['*']")}
      >
        {t(`fields.${f}`)}
      </FieldLabel>
      {input}
      {opts.hint && !errors[f] && <FieldDescription>{opts.hint}</FieldDescription>}
      <FieldError>{errorText(f)}</FieldError>
    </Field>
  )
  const dateField = (f: "date" | "next_date", required: boolean) =>
    field(
      f,
      <Controller
        control={control}
        name={f}
        render={({ field: c }) => (
          <DateInput
            id={id(f)}
            invalid={!!errors[f]}
            value={c.value}
            onChange={c.onChange}
            onBlur={c.onBlur}
            clearable={!required}
          />
        )}
      />,
      { required },
    )

  const days = /^\d+$/.test(reminderDays.trim()) ? reminderDays.trim() : DEFAULT_REMINDER_DAYS

  return (
    <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {dateField("date", true)}
        {field(
          "type",
          <Controller
            control={control}
            name="type"
            render={({ field: c }) => (
              <Select value={c.value} onValueChange={c.onChange}>
                <SelectTrigger
                  id={id("type")}
                  ref={c.ref}
                  onBlur={c.onBlur}
                  aria-invalid={!!errors.type}
                  aria-required
                  className="w-full"
                >
                  <SelectValue placeholder={t("typePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {MAINTENANCE_TYPES.map((code) => (
                    <SelectItem key={code} value={code}>
                      {tType(code)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />,
          { required: true },
        )}
        {field("performed_by", <Input id={id("performed_by")} autoComplete="off" {...register("performed_by")} />)}
        {field(
          "cost",
          <div className="relative">
            <span className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-subtle-foreground" aria-hidden>
              Rp
            </span>
            <Input
              id={id("cost")}
              aria-invalid={!!errors.cost}
              inputMode="decimal"
              autoComplete="off"
              className="pl-12"
              {...register("cost")}
            />
          </div>,
        )}
        {dateField("next_date", false)}
        {nextDate &&
          field(
            "reminder_days",
            <Input
              id={id("reminder_days")}
              aria-invalid={!!errors.reminder_days}
              inputMode="numeric"
              autoComplete="off"
              {...register("reminder_days")}
            />,
            { hint: t("reminderHint", { days }) },
          )}
        {field("notes", <Textarea id={id("notes")} rows={3} {...register("notes")} />, { wide: true })}
      </div>
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
