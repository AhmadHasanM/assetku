"use client"

import { useCallback, useEffect, useId, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useFormatter, useTranslations } from "next-intl"
import { toast } from "sonner"
import { CircleAlert, Download, FileSpreadsheet, Loader2, Upload } from "lucide-react"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Progress } from "@/components/ui/progress"
import { api, ApiError } from "@/lib/api"
import type { ImportJob } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { ImportResult } from "./import-result"
import {
  clearImportResult,
  closeImportDialog,
  dropImportJob,
  finishImportJob,
  getImportState,
  showImportResult,
  startImportJob,
  useImportState,
} from "./import-store"

const MAX_BYTES = 10 * 1024 * 1024
const ACCEPT = ".xlsx,.xls,.csv"
const POLL_MS = 1000

const isFinal = (s: ImportJob["status"] | undefined) => s === "done" || s === "failed"

/** Client-side checks mirroring the API, so an oversized file never leaves the browser. */
function checkFile(file: File): string | null {
  const ext = file.name.toLowerCase().slice(file.name.lastIndexOf("."))
  if (!ACCEPT.split(",").includes(ext)) return "unsupported_format"
  if (file.size > MAX_BYTES) return "file_too_large"
  return null
}

/**
 * The single import dialog and job poller, mounted in the app shell. Polling
 * continues while the dialog is closed (and resumes from sessionStorage after a
 * reload); a job finishing in the background is announced with a toast.
 */
export function ImportDialog() {
  const t = useTranslations("import")
  const tErr = useTranslations("errors")
  const tc = useTranslations("common")
  const qc = useQueryClient()
  const { open, jobId, result } = useImportState()

  const job = useQuery({
    queryKey: ["imports", jobId],
    queryFn: () => api<ImportJob>(`/api/imports/${jobId}`),
    enabled: !!jobId,
    staleTime: 0,
    refetchInterval: (q) => (isFinal(q.state.data?.status) ? false : POLL_MS),
  })

  const jobFailedCode = job.error instanceof ApiError ? job.error.status : 0
  useEffect(() => {
    // The job is gone (another session, or the server lost it): stop polling.
    if (jobId && jobFailedCode === 404) dropImportJob()
  }, [jobId, jobFailedCode])

  const failureText = useCallback(
    (j: ImportJob) => {
      const key = `jobErrors.${j.error}`
      return t.has(key) ? t(key) : tErr("generic")
    },
    [t, tErr],
  )

  const data = job.data
  useEffect(() => {
    if (!jobId || !data || data.id !== jobId || !isFinal(data.status)) return
    const inBackground = !getImportState().open
    finishImportJob(data)
    // New assets, suppliers and locations; prefix matches cover lists and summaries.
    for (const key of ["assets", "locations", "suppliers", "dashboard"]) qc.invalidateQueries({ queryKey: [key] })
    if (!inBackground) return
    const action = { label: t("viewResult"), onClick: () => showImportResult(data) }
    if (data.status === "done" && data.result) {
      toast.success(
        t("toastDone", { created: data.result.created, updated: data.result.updated, skipped: data.result.skipped }),
        { action, duration: 10_000 },
      )
    } else {
      toast.error(t("toastFailed", { reason: failureText(data) }), { action })
    }
  }, [jobId, data, qc, t, failureText])

  const onOpenChange = (next: boolean) => {
    if (next) return
    if (result) for (const key of ["assets", "locations", "suppliers", "dashboard"]) qc.invalidateQueries({ queryKey: [key] })
    closeImportDialog()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-[22px] font-semibold">{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {result ? (
          result.status === "done" && result.result ? (
            <>
              <ImportResult jobId={result.id} result={result.result} />
              <DialogFooter>
                <Button variant="ghost" onClick={clearImportResult}>
                  {t("tryAgain")}
                </Button>
                <Button variant="secondary" onClick={() => onOpenChange(false)}>
                  {tc("close")}
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div role="alert" className="flex items-start gap-3 rounded-xl border border-destructive/40 p-4">
                <CircleAlert className="mt-0.5 shrink-0 text-destructive" />
                <div>
                  <p className="font-semibold">{t("failedTitle")}</p>
                  <p className="text-sm text-muted-foreground">{failureText(result)}</p>
                </div>
              </div>
              <DialogFooter>
                <Button variant="secondary" onClick={clearImportResult}>
                  {t("tryAgain")}
                </Button>
              </DialogFooter>
            </>
          )
        ) : jobId ? (
          <ProgressStep job={data?.id === jobId ? data : undefined} />
        ) : (
          <UploadStep />
        )}
      </DialogContent>
    </Dialog>
  )
}

function ProgressStep({ job }: { job: ImportJob | undefined }) {
  const t = useTranslations("import")
  const format = useFormatter()
  const total = job?.total_rows ?? 0
  const processed = Math.min(job?.processed_rows ?? 0, total)
  const pct = total > 0 ? Math.floor((processed / total) * 100) : 0
  return (
    <div className="flex flex-col gap-3" aria-live="polite">
      <div className="flex items-baseline justify-between gap-3">
        <p className="font-semibold">{t("processing")}</p>
        <p className="font-heading text-xl font-semibold">{format.number(pct)}%</p>
      </div>
      <Progress
        value={pct}
        aria-label={t("progressLabel")}
        className="h-2 [&>[data-slot=progress-indicator]]:bg-nebula"
      />
      <p className="text-sm text-muted-foreground">{t("progressRows", { processed, total })}</p>
      <p className="text-sm text-muted-foreground">{t("closeHint")}</p>
    </div>
  )
}

function UploadStep() {
  const t = useTranslations("import")
  const tErr = useTranslations("errors")
  const errorMessage = useErrorMessage()
  const inputId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  const upload = useMutation({
    mutationFn: (f: File) => {
      const body = new FormData()
      body.append("file", f)
      return api<{ id: string }>("/api/imports", { method: "POST", body })
    },
    onSuccess: ({ id }) => startImportJob(id),
    onError: (err) => setError(errorMessage(err)),
  })

  const pick = (f: File | undefined) => {
    if (!f) return
    const code = checkFile(f)
    setFile(code ? null : f)
    setError(code ? tErr(code) : null)
    // Let the same file be chosen again after an error.
    if (inputRef.current) inputRef.current.value = ""
  }

  return (
    <>
      <label
        htmlFor={inputId}
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragging(false)
          pick(e.dataTransfer.files[0])
        }}
        className={cn(
          "flex cursor-pointer flex-col items-center gap-2 rounded-2xl border-2 border-dashed p-6 text-center transition-colors hover:border-nebula-text focus-within:border-nebula-text",
          dragging && "border-nebula-text bg-nebula/10",
          error && "border-destructive",
        )}
      >
        {file ? <FileSpreadsheet className="size-8 text-nebula-text" /> : <Upload className="size-8 text-nebula-text" />}
        <span className="font-semibold break-all">{file ? file.name : t("dropTitle")}</span>
        <span className="text-sm text-muted-foreground">{t("dropHint")}</span>
        <input
          ref={inputRef}
          id={inputId}
          type="file"
          accept={ACCEPT}
          aria-label={t("chooseFile")}
          className="sr-only"
          onChange={(e) => pick(e.target.files?.[0])}
        />
      </label>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter className="items-center sm:justify-between">
        <Button variant="ghost" asChild>
          <a href="/api/imports/template.xlsx" download>
            <Download />
            {t("template")}
          </a>
        </Button>
        <Button disabled={!file || upload.isPending} onClick={() => file && upload.mutate(file)}>
          {upload.isPending ? <Loader2 className="animate-spin" /> : <Upload />}
          {upload.isPending ? t("uploading") : t("start")}
        </Button>
      </DialogFooter>
    </>
  )
}
