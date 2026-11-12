"use client"

import { useFormatter, useTranslations } from "next-intl"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { cardStatuses, type SummaryCard } from "@/lib/filters"

export interface AssetSummary {
  total: number
  available: number
  in_use: number
  needs_attention: number
}

const CARDS: readonly { key: SummaryCard; value: keyof AssetSummary }[] = [
  { key: "total", value: "total" },
  { key: "available", value: "available" },
  { key: "in_use", value: "in_use" },
  { key: "attention", value: "needs_attention" },
]

interface Props {
  summary: AssetSummary | undefined
  /** Card that matches the current status filter, if any. */
  active: SummaryCard | null
  /** Called with the status filter the clicked card stands for (empty for Total). */
  onChange: (statuses: string[]) => void
}

/** Four tenant-wide counts; each card doubles as a status filter. */
export function SummaryCards({ summary, active, onChange }: Props) {
  const t = useTranslations("assets.summary")
  const format = useFormatter()
  return (
    <div role="group" aria-label={t("label")} className="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
      {CARDS.map(({ key, value }) => {
        const pressed = active === key
        return (
          <button
            key={key}
            type="button"
            aria-pressed={pressed}
            onClick={() => onChange(cardStatuses(key))}
            className={cn(
              "flex min-h-24 flex-col items-start justify-between gap-2 rounded-lg border bg-card p-4 text-left transition-shadow outline-none md:p-5",
              "hover:border-nebula-text focus-visible:glow-nebula-sm",
              pressed && "border-nebula-text glow-nebula-md",
            )}
          >
            <span className="text-sm font-semibold text-muted-foreground">{t(key)}</span>
            {summary ? (
              <span className="font-heading text-3xl leading-none font-semibold tabular-nums">{format.number(summary[value])}</span>
            ) : (
              <Skeleton className="h-8 w-14" />
            )}
          </button>
        )
      })}
    </div>
  )
}
