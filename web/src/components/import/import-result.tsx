"use client"

import { useState } from "react"
import { useFormatter, useTranslations } from "next-intl"
import { toast } from "sonner"
import { CheckCircle2, ChevronDown, Download, Loader2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { downloadBlob } from "@/components/assets/detail/download"
import { apiBlob } from "@/lib/api"
import type { ImportResultData } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"

const COUNTS = ["created", "updated", "unchanged", "skipped"] as const

/** Translates a skipped-row reason (missing_name, duplicate_tag_in_file, invalid_value:<field>, error). */
export function useSkipReason() {
  const t = useTranslations("import.reasons")
  const tf = useTranslations("assetForm.fields")
  return (reason: string): string => {
    if (reason.startsWith("invalid_value:")) {
      const field = reason.slice("invalid_value:".length)
      return t("invalid_value", { field: tf.has(field) ? tf(field) : field })
    }
    return t.has(reason) ? t(reason) : t("error")
  }
}

function Section({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  if (count === 0) return null
  return (
    <Collapsible className="rounded-xl border">
      <CollapsibleTrigger asChild>
        <Button variant="ghost" className="group w-full justify-between rounded-xl px-4 text-foreground">
          <span className="flex items-center gap-2">
            {title}
            <Badge variant="secondary">{count}</Badge>
          </span>
          <ChevronDown className="transition-transform group-data-[state=open]:rotate-180" />
        </Button>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul className="flex max-h-48 flex-col gap-1 overflow-y-auto px-4 pb-3 text-sm">{children}</ul>
      </CollapsibleContent>
    </Collapsible>
  )
}

function Line({ label, detail }: { label: string; detail?: string }) {
  return (
    <li className="flex justify-between gap-3 border-b border-border/50 py-1.5 last:border-0">
      <span className="min-w-0 break-words">{label}</span>
      {detail && <span className="text-right text-muted-foreground">{detail}</span>}
    </li>
  )
}

/** Step 3 of the import dialog: the four counts, collapsible details and the CSV report. */
export function ImportResult({ jobId, result }: { jobId: string; result: ImportResultData }) {
  const t = useTranslations("import")
  const tf = useTranslations("assetForm.fields")
  const format = useFormatter()
  const skipReason = useSkipReason()
  const errorMessage = useErrorMessage()
  const [downloading, setDownloading] = useState(false)

  const rows = (count: number) => t("rows", { count })
  const fieldLabel = (f: string) => (tf.has(f) ? tf(f) : f)

  const downloadReport = async () => {
    setDownloading(true)
    try {
      downloadBlob(await apiBlob(`/api/imports/${jobId}/report.csv`), `import-report-${jobId}.csv`)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        {/* The one "achievement" glow of the import flow (spec §3: used sparingly). */}
        <span className="flex size-11 shrink-0 items-center justify-center rounded-full border-2 border-star text-star-text glow-star-md animate-in zoom-in-50 duration-500">
          <CheckCircle2 />
        </span>
        <p className="font-heading text-xl font-semibold">{t("doneTitle")}</p>
      </div>

      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {COUNTS.map((key) => (
          <div key={key} data-testid={`count-${t(`counts.${key}`)}`} className="rounded-2xl border p-3">
            <dt className="text-sm text-muted-foreground">{t(`counts.${key}`)}</dt>
            <dd className="font-heading text-2xl font-semibold">{format.number(result[key])}</dd>
          </div>
        ))}
      </dl>

      <div className="flex flex-col gap-2">
        <Section title={t("sections.unknownColumns")} count={result.unknown_columns.length}>
          {result.unknown_columns.map((c) => (
            <Line key={c} label={c} />
          ))}
        </Section>
        <Section title={t("sections.unmatchedOrganizations")} count={result.unmatched_organizations.length}>
          {result.unmatched_organizations.map((o) => (
            <Line key={o.name} label={o.name} detail={rows(o.rows)} />
          ))}
        </Section>
        <Section title={t("sections.unrecognizedValues")} count={result.unrecognized_values.length}>
          {result.unrecognized_values.map((v) => (
            <Line
              key={`${v.field}:${v.value}`}
              label={`${fieldLabel(v.field)}: ${v.value === "" ? t("emptyValue") : v.value}`}
              detail={rows(v.rows)}
            />
          ))}
        </Section>
        <Section title={t("sections.createdSuppliers")} count={result.created_suppliers.length}>
          {result.created_suppliers.map((s) => (
            <Line key={s} label={s} />
          ))}
        </Section>
        <Section title={t("sections.createdLocations")} count={result.created_locations.length}>
          {result.created_locations.map((l) => (
            <Line key={l} label={l} />
          ))}
        </Section>
        <Section title={t("sections.skippedRows")} count={result.skipped_rows.length}>
          {result.skipped_rows.map((r) => (
            <Line key={r.row} label={t("row", { row: r.row })} detail={skipReason(r.reason)} />
          ))}
        </Section>
      </div>

      <Button variant="secondary" className="self-start" onClick={downloadReport} disabled={downloading}>
        {downloading ? <Loader2 className="animate-spin" /> : <Download />}
        {t("downloadReport")}
      </Button>
    </div>
  )
}
