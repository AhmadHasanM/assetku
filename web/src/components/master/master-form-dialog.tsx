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
import { Textarea } from "@/components/ui/textarea"
import { applyFieldErrors } from "@/lib/form-errors"
import { useFieldErrorMessage } from "@/lib/use-error-message"
import { KIND_CONFIG, type MasterField, type MasterItem, type MasterKind } from "./kinds"

const schema = z.object({
  name: z.string().trim().min(1, { error: "required" }),
  description: z.string(),
  contact_name: z.string(),
  phone: z.string(),
  email: z.string().refine((v) => v.trim() === "" || /^\S+@\S+\.\S+$/.test(v.trim()), { error: "invalid" }),
  address: z.string(),
})
export type MasterValues = z.infer<typeof schema>

const EMPTY: MasterValues = { name: "", description: "", contact_name: "", phone: "", email: "", address: "" }
const MULTILINE: readonly MasterField[] = ["description", "address"]

interface Props {
  kind: MasterKind
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Item being edited, or null when adding. */
  item: MasterItem | null
  /** Throws the ApiError when the save fails; field errors are shown inline. */
  onSubmit: (values: Partial<MasterValues>) => Promise<void>
}

/** Add/edit dialog for a master data row. */
export function MasterFormDialog({ kind, open, onOpenChange, item, onSubmit }: Props) {
  const t = useTranslations("master")
  const tc = useTranslations("common")
  const fieldMessage = useFieldErrorMessage()
  const config = KIND_CONFIG[kind]
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<MasterValues>({ resolver: zodResolver(schema), defaultValues: EMPTY })

  useEffect(() => {
    if (!open) return
    reset({
      ...EMPTY,
      ...Object.fromEntries(config.fields.map((f) => [f, (f === "name" ? item?.name : item?.[f]) ?? ""])),
    })
  }, [open, item, config, reset])

  const submit = async (values: MasterValues) => {
    const body: Partial<MasterValues> = {}
    for (const f of config.fields) body[f] = values[f].trim()
    try {
      await onSubmit(body)
    } catch (err) {
      // Anything not shown inline was already reported by the caller.
      applyFieldErrors(err, setError, config.fields)
    }
  }

  const title = t(`kinds.${kind}.${item ? "editTitle" : "addTitle"}`)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{title}</DialogTitle>
          <DialogDescription className="sr-only">{title}</DialogDescription>
        </DialogHeader>
        <form noValidate onSubmit={handleSubmit(submit)} className="flex flex-col gap-5">
          <FieldGroup className="gap-4">
            {config.fields.map((f) => {
              const err = errors[f]
              const Control = MULTILINE.includes(f) ? Textarea : Input
              return (
                <Field key={f} data-invalid={!!err}>
                  <FieldLabel htmlFor={`master-${f}`}>
                    {t(`fields.${f}`)}
                    {f !== "name" && <span className="ml-1 font-normal text-subtle-foreground">({tc("optional")})</span>}
                  </FieldLabel>
                  <Control
                    id={`master-${f}`}
                    aria-invalid={!!err}
                    autoComplete="off"
                    {...(f === "email" ? { type: "email", inputMode: "email" as const } : {})}
                    {...(f === "phone" ? { type: "tel", inputMode: "tel" as const } : {})}
                    {...register(f)}
                  />
                  <FieldError>{err ? fieldMessage(err.message || err.type) : undefined}</FieldError>
                </Field>
              )
            })}
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
