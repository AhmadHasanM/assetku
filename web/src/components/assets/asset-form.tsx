"use client"

import { useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { Controller, useForm, useWatch, type Control } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { toast } from "sonner"
import { cn } from "cn"
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
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { api, ApiError } from "@/lib/api"
import { invalidateAssetData } from "@/lib/invalidate"
import { formatDate } from "@/lib/format"
import { ASSET_CATEGORIES, ASSET_STATUSES, type Asset, type Locale } from "@/lib/types"
import { useErrorMessage, useFieldErrorMessage } from "@/lib/use-error-message"
import { AssigneeInput } from "./assignee-input"
import {
  API_FIELD_TO_FORM,
  EMPTY_ASSET,
  assetSchema,
  fromAsset,
  toAssetInput,
  warrantyExpiresPreview,
  type AssetFormField,
  type AssetFormValues,
} from "./asset-schema"
import { DateInput } from "./date-input"
import { MasterCombobox } from "./master-combobox"

/**
 * Warns before leaving while dirty: beforeunload for reload/close, and a
 * capture-phase click listener that holds same-origin link clicks (sidebar,
 * breadcrumbs) for confirmation. The App Router has no route-change event.
 */
function useLeaveGuard(dirty: boolean) {
  const router = useRouter()
  const [pending, setPending] = useState<string | null>(null)
  const allowed = useRef(false)

  useEffect(() => {
    if (!dirty) return
    const beforeUnload = (e: BeforeUnloadEvent) => {
      if (allowed.current) return
      e.preventDefault()
      e.returnValue = ""
    }
    const click = (e: MouseEvent) => {
      if (allowed.current || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
      const a = e.target instanceof Element ? e.target.closest("a[href]") : null
      if (!(a instanceof HTMLAnchorElement) || (a.target && a.target !== "_self") || a.hasAttribute("download")) return
      const url = new URL(a.href, window.location.href)
      // Other sites go through beforeunload; same-page links do not leave.
      if (url.origin !== window.location.origin) return
      if (url.pathname === window.location.pathname && url.search === window.location.search) return
      e.preventDefault()
      e.stopPropagation()
      setPending(url.pathname + url.search + url.hash)
    }
    window.addEventListener("beforeunload", beforeUnload)
    document.addEventListener("click", click, true)
    return () => {
      window.removeEventListener("beforeunload", beforeUnload)
      document.removeEventListener("click", click, true)
    }
  }, [dirty])

  const go = (href: string) => {
    allowed.current = true
    router.push(href)
  }
  return {
    pending,
    /** Navigates now when clean, otherwise asks first. */
    leave: (href: string) => (dirty ? setPending(href) : go(href)),
    /** Navigates without asking (after a successful save). */
    go,
    confirm: () => {
      if (pending) go(pending)
      setPending(null)
    },
    stay: () => setPending(null),
  }
}

function WarrantyPreview({ control }: { control: Control<AssetFormValues> }) {
  const t = useTranslations("assetForm")
  const locale = useLocale() as Locale
  const [date, months] = useWatch({ control, name: ["purchase_date", "warranty_months"] })
  const m = months.trim()
  const preview = /^\d+$/.test(m) ? warrantyExpiresPreview(date, Number(m)) : null
  return (
    <Field>
      <FieldLabel id="asset-warranty_expires-label">{t("fields.warranty_expires")}</FieldLabel>
      <output
        aria-labelledby="asset-warranty_expires-label"
        aria-live="polite"
        className="flex min-h-12 items-center rounded-md border-2 border-dashed border-border px-4 text-base"
      >
        {preview ? formatDate(preview, locale) : <span className="text-sm text-subtle-foreground">{t("hints.warrantyExpires")}</span>}
      </output>
    </Field>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2 className="text-[22px] font-semibold">{title}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-5 md:grid-cols-2">{children}</CardContent>
    </Card>
  )
}

const REQUIRED: readonly AssetFormField[] = ["name", "category", "status"]

/** Add (no asset) or edit form. Saves, then opens the asset's detail page. */
export function AssetForm({ asset }: { asset?: Asset }) {
  const t = useTranslations("assetForm")
  const tc = useTranslations("common")
  const tCat = useTranslations("category")
  const tStatus = useTranslations("status")
  const errorMessage = useErrorMessage()
  const fieldMessage = useFieldErrorMessage()
  const qc = useQueryClient()

  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<AssetFormValues>({
    resolver: zodResolver(assetSchema),
    defaultValues: asset ? fromAsset(asset) : EMPTY_ASSET,
  })
  const guard = useLeaveGuard(isDirty && !isSubmitting)
  const cancelHref = asset ? `/assets/${asset.id}` : "/assets"

  const save = useMutation({
    mutationFn: (values: AssetFormValues) => {
      const json = toAssetInput(values, asset?.asset_tag)
      return asset
        ? api<Asset>(`/api/assets/${asset.id}`, { method: "PUT", json })
        : api<Asset>("/api/assets", { method: "POST", json })
    },
  })

  const onSubmit = async (values: AssetFormValues) => {
    try {
      const saved = await save.mutateAsync(values)
      invalidateAssetData(qc)
      toast.success(t("saved"))
      guard.go(`/assets/${saved.id}`)
    } catch (err) {
      let shown = false
      if (err instanceof ApiError) {
        for (const [apiField, code] of Object.entries(err.fields)) {
          const field = API_FIELD_TO_FORM[apiField]
          if (!field) continue
          setError(field, { type: code }, { shouldFocus: !shown })
          shown = true
        }
      }
      if (!shown) toast.error(errorMessage(err))
    }
  }

  const fieldId = (f: AssetFormField) => `asset-${f}`
  const errorText = (f: AssetFormField) => {
    const e = errors[f]
    return e ? fieldMessage(e.message || e.type) : undefined
  }

  /** A labelled field; the control receives id, aria-invalid and aria-required. */
  const field = (
    f: AssetFormField,
    render: (a11y: { id: string; "aria-invalid": boolean; "aria-required"?: boolean }) => React.ReactNode,
    opts: { hint?: string; wide?: boolean } = {},
  ) => {
    const invalid = !!errors[f]
    const required = REQUIRED.includes(f)
    return (
      <Field data-invalid={invalid} className={cn(opts.wide && "md:col-span-2")}>
        <FieldLabel
          htmlFor={fieldId(f)}
          className={cn(required && "after:-ml-1 after:text-error after:content-['*']")}
        >
          {t(`fields.${f}`)}
        </FieldLabel>
        {render({ id: fieldId(f), "aria-invalid": invalid, ...(required ? { "aria-required": true } : {}) })}
        {opts.hint && !invalid && <FieldDescription>{opts.hint}</FieldDescription>}
        <FieldError>{errorText(f)}</FieldError>
      </Field>
    )
  }

  const text = (f: AssetFormField, opts: { hint?: string; mono?: boolean } = {}) =>
    field(
      f,
      (a11y) => (
        <Input {...a11y} autoComplete="off" className={cn(opts.mono && "font-mono")} {...register(f)} />
      ),
      opts,
    )

  const enumSelect = (
    f: "category" | "status",
    codes: readonly string[],
    label: (c: string) => string,
    placeholder?: string,
  ) =>
    field(f, (a11y) => (
      <Controller
        control={control}
        name={f}
        render={({ field: c }) => (
          <Select value={c.value} onValueChange={c.onChange}>
            <SelectTrigger {...a11y} ref={c.ref} onBlur={c.onBlur} className="w-full">
              <SelectValue placeholder={placeholder} />
            </SelectTrigger>
            <SelectContent>
              {codes.map((code) => (
                <SelectItem key={code} value={code}>
                  {label(code)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
    ))

  const master = (f: "location" | "organization" | "supplier", kind: "locations" | "organizations" | "suppliers") =>
    field(f, (a11y) => (
      <Controller
        control={control}
        name={f}
        render={({ field: c }) => (
          <MasterCombobox
            kind={kind}
            id={a11y.id}
            invalid={a11y["aria-invalid"]}
            value={c.value}
            onChange={c.onChange}
            onBlur={c.onBlur}
            placeholder={t("placeholders.master")}
          />
        )}
      />
    ))

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <h1 className="text-[28px] sm:text-4xl">{asset ? t("editTitle") : t("addTitle")}</h1>
      <form noValidate onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-6">
        <Section title={t("sections.general")}>
          {text("name")}
          {text("asset_tag", { mono: true, hint: asset ? t("hints.assetTagEdit") : t("hints.assetTagNew") })}
          {enumSelect("category", ASSET_CATEGORIES, (c) => tCat(c), t("placeholders.category"))}
          {text("manufacturer")}
          {text("model")}
          {text("serial_number", { mono: true })}
        </Section>

        <Section title={t("sections.placement")}>
          {enumSelect("status", ASSET_STATUSES, (s) => tStatus(s))}
          {master("location", "locations")}
          {field("assigned_to", (a11y) => (
            <Controller
              control={control}
              name="assigned_to"
              render={({ field: c }) => (
                <AssigneeInput
                  {...a11y}
                  ref={c.ref}
                  name={c.name}
                  value={c.value}
                  onChange={c.onChange}
                  onBlur={c.onBlur}
                  placeholder={t("placeholders.assignee")}
                />
              )}
            />
          ))}
          {master("organization", "organizations")}
        </Section>

        <Section title={t("sections.purchase")}>
          {master("supplier", "suppliers")}
          {text("order_number")}
          {field("purchase_date", (a11y) => (
            <Controller
              control={control}
              name="purchase_date"
              render={({ field: c }) => (
                <DateInput id={a11y.id} invalid={a11y["aria-invalid"]} value={c.value} onChange={c.onChange} onBlur={c.onBlur} />
              )}
            />
          ))}
          {field(
            "purchase_cost",
            (a11y) => (
              <div className="relative">
                <span className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-subtle-foreground" aria-hidden>
                  Rp
                </span>
                <Input {...a11y} inputMode="decimal" autoComplete="off" className="pl-12" {...register("purchase_cost")} />
              </div>
            ),
            { hint: t("hints.purchaseCost") },
          )}
          {field("warranty_months", (a11y) => (
            <Input {...a11y} inputMode="numeric" autoComplete="off" {...register("warranty_months")} />
          ))}
          <WarrantyPreview control={control} />
        </Section>

        <Section title={t("sections.notes")}>
          {field("notes", (a11y) => <Textarea {...a11y} rows={4} {...register("notes")} />, { wide: true })}
        </Section>

        <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button type="button" variant="ghost" onClick={() => guard.leave(cancelHref)}>
            {tc("cancel")}
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? tc("saving") : tc("save")}
          </Button>
        </div>
      </form>

      <AlertDialog open={guard.pending !== null} onOpenChange={(open) => !open && guard.stay()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("leave.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("leave.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("leave.stay")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={guard.confirm}>
              {t("leave.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
