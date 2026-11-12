"use client"

import { useState } from "react"
import { useLocale, useTranslations } from "next-intl"
import { CalendarDays } from "lucide-react"
import { enUS, id as idLocale } from "react-day-picker/locale"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { formatDate } from "@/lib/format"
import type { Locale } from "@/lib/types"
import { INPUT_TRIGGER_CLASS, PLACEHOLDER_CLASS } from "./master-combobox"

const pad = (n: number) => String(n).padStart(2, "0")
const toIso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
function fromIso(s: string): Date | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s)
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : undefined
}

export interface DateInputProps {
  id: string
  value: string
  onChange: (v: string) => void
  onBlur: () => void
  invalid: boolean
  /** Offer a "clear" button (default true); off for required dates. */
  clearable?: boolean
}

/** Date picker showing "dd MMM yyyy"; the value is yyyy-mm-dd or "". */
export function DateInput({ id, value, onChange, onBlur, invalid, clearable = true }: DateInputProps) {
  const t = useTranslations("assetForm")
  const locale = useLocale() as Locale
  const [open, setOpen] = useState(false)
  const selected = fromIso(value)
  const thisYear = new Date().getFullYear()
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) onBlur()
      }}
    >
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="ghost"
          aria-invalid={invalid || undefined}
          className={cn(INPUT_TRIGGER_CLASS, !value && PLACEHOLDER_CLASS)}
        >
          <span className="truncate">{value ? formatDate(value, locale) : t("placeholders.date")}</span>
          <CalendarDays className="text-muted-foreground" aria-hidden />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-auto p-2">
        <Calendar
          mode="single"
          captionLayout="dropdown"
          // 44px day cells: tap target minimum (7 x 44px still fits a 360px screen).
          className="[--cell-size:--spacing(11)]"
          locale={locale === "en" ? enUS : idLocale}
          selected={selected}
          defaultMonth={selected}
          startMonth={new Date(thisYear - 40, 0)}
          endMonth={new Date(thisYear + 5, 11)}
          onSelect={(d) => {
            onChange(d ? toIso(d) : "")
            setOpen(false)
          }}
        />
        {clearable && value && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              onChange("")
              setOpen(false)
            }}
          >
            {t("clearDate")}
          </Button>
        )}
      </PopoverContent>
    </Popover>
  )
}
