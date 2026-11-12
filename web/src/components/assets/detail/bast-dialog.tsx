"use client"

import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { api, apiBlob } from "@/lib/api"
import { applyFieldErrors } from "@/lib/form-errors"
import { BAST_CONDITIONS } from "@/lib/types"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"
import { isIsoDate } from "../asset-schema"
import { DateInput } from "../date-input"
import { downloadBlob } from "./download"

/** GET /api/assets/{id}/bast/defaults. */
interface BastDefaults {
  number: string
  date: string
  first_name: string
  first_position: string
  second_name: string
  warranty: string
}

const schema = z.object({
  date: z.string().refine(isIsoDate, { error: "required" }),
  first_name: z.string(),
  first_position: z.string(),
  second_name: z.string().refine((v) => v.trim() !== "", { error: "required" }),
  second_position: z.string(),
  second_department: z.string(),
  condition: z.enum(BAST_CONDITIONS, { error: "required" }),
  warranty: z.string(),
  notes: z.string(),
})
type BastValues = z.infer<typeof schema>
type BastField = keyof BastValues
const FIELDS = Object.keys(schema.shape) as BastField[]

/** BAST-<tag>-<YYYYMMDD>, the same rule as the API. */
export function bastNumber(tag: string, isoDate: string): string {
  return `BAST-${tag}-${isoDate.replaceAll("-", "")}`
}

interface AssetRef {
  id: string
  asset_tag: string
}

interface Props {
  asset: AssetRef
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** "Print BAST": a handover form prefilled from the API that downloads the A4 PDF. */
export function BastDialog({ asset, open, onOpenChange }: Props) {
  const t = useTranslations("assetDetail.bast")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {/* Content unmounts on close, so every open fetches fresh defaults and starts a clean form. */}
        <BastBody asset={asset} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  )
}

function BastBody({ asset, onDone }: { asset: AssetRef; onDone: () => void }) {
  const tc = useTranslations("common")
  const errorMessage = useErrorMessage()
  const defaults = useQuery({
    queryKey: ["assets", "detail", asset.id, "bast-defaults"],
    queryFn: () => api<BastDefaults>(`/api/assets/${asset.id}/bast/defaults`),
    staleTime: 0,
    gcTime: 0,
  })
  if (defaults.data && defaults.isFetchedAfterMount) {
    return <BastForm asset={asset} defaults={defaults.data} onDone={onDone} />
  }
  if (defaults.isError) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p role="alert">{errorMessage(defaults.error)}</p>
        <Button variant="secondary" onClick={() => defaults.refetch()}>
          {tc("retry")}
        </Button>
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-4" aria-busy="true">
      <Skeleton className="h-12 rounded-md" />
      <Skeleton className="h-12 rounded-md" />
      <Skeleton className="h-12 rounded-md" />
    </div>
  )
}

function BastForm({ asset, defaults, onDone }: { asset: AssetRef; defaults: BastDefaults; onDone: () => void }) {
  const t = useTranslations("assetDetail.bast")
  const tc = useTranslations("common")
  const tCondition = useTranslations("condition")
  const tActions = useTranslations("assetDetail.actions")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const qc = useQueryClient()
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<BastValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      date: defaults.date,
      first_name: defaults.first_name,
      first_position: defaults.first_position,
      second_name: defaults.second_name,
      second_position: "",
      second_department: "",
      condition: "good",
      warranty: defaults.warranty,
      notes: "",
    },
  })
  const date = useWatch({ control, name: "date" })
  const number = isIsoDate(date) ? bastNumber(asset.asset_tag, date) : "—"

  const submit = async (values: BastValues) => {
    try {
      const blob = await apiBlob(`/api/assets/${asset.id}/bast`, { method: "POST", json: values })
      downloadBlob(blob, `${bastNumber(asset.asset_tag, values.date)}.pdf`)
      qc.invalidateQueries({ queryKey: ["assets", "detail", asset.id, "history"] })
      toast.success(t("done"))
      onDone()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) toast.error(errorMessage(err))
    }
  }

  const id = (f: BastField) => `bast-${f}`
  const errorText = (f: BastField) => {
    const e = errors[f]
    return e ? fieldMessage(e.message || e.type) : undefined
  }
  const text = (f: BastField, opts: { required?: boolean } = {}) => (
    <Field data-invalid={!!errors[f]}>
      <FieldLabel
        htmlFor={id(f)}
        className={opts.required ? "after:-ml-1 after:text-error after:content-['*']" : undefined}
      >
        {t(f)}
      </FieldLabel>
      <Input
        id={id(f)}
        aria-invalid={!!errors[f]}
        aria-required={opts.required || undefined}
        autoComplete="off"
        {...register(f)}
      />
      <FieldError>{errorText(f)}</FieldError>
    </Field>
  )

  return (
    <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field>
          <FieldLabel id="bast-number-label">{t("number")}</FieldLabel>
          <output
            aria-labelledby="bast-number-label"
            aria-live="polite"
            className="flex min-h-12 items-center rounded-md border-2 border-dashed border-border px-4 font-mono text-sm break-all"
          >
            {number}
          </output>
        </Field>
        <Field data-invalid={!!errors.date}>
          <FieldLabel htmlFor={id("date")}>{t("date")}</FieldLabel>
          <Controller
            control={control}
            name="date"
            render={({ field: c }) => (
              <DateInput
                id={id("date")}
                invalid={!!errors.date}
                value={c.value}
                onChange={c.onChange}
                onBlur={c.onBlur}
                clearable={false}
              />
            )}
          />
          <FieldError>{errorText("date")}</FieldError>
        </Field>
      </div>

      <FieldSet className="gap-4">
        <FieldLegend className="font-heading text-lg font-semibold">{t("firstParty")}</FieldLegend>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {text("first_name")}
          {text("first_position")}
        </div>
      </FieldSet>

      <FieldSet className="gap-4">
        <FieldLegend className="font-heading text-lg font-semibold">{t("secondParty")}</FieldLegend>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {text("second_name", { required: true })}
          {text("second_position")}
          {text("second_department")}
        </div>
      </FieldSet>

      <FieldSet className="gap-2" data-invalid={!!errors.condition}>
        <FieldLegend className="font-heading text-lg font-semibold">{t("condition")}</FieldLegend>
        <Controller
          control={control}
          name="condition"
          render={({ field: c }) => (
            <RadioGroup value={c.value} onValueChange={c.onChange} aria-invalid={!!errors.condition} className="gap-0">
              {BAST_CONDITIONS.map((code) => (
                <Label key={code} htmlFor={`bast-condition-${code}`} className="flex min-h-11 items-center gap-3 font-normal">
                  <RadioGroupItem id={`bast-condition-${code}`} value={code} />
                  {tCondition(code)}
                </Label>
              ))}
            </RadioGroup>
          )}
        />
        <FieldError>{errorText("condition")}</FieldError>
      </FieldSet>

      {text("warranty")}
      <Field>
        <FieldLabel htmlFor={id("notes")}>{t("notes")}</FieldLabel>
        <Textarea id={id("notes")} rows={3} {...register("notes")} />
      </Field>

      <DialogFooter className="-mx-0 -mb-0 border-0 bg-transparent p-0">
        <Button type="button" variant="secondary" onClick={onDone}>
          {tc("cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? tActions("generating") : t("submit")}
        </Button>
      </DialogFooter>
    </form>
  )
}
