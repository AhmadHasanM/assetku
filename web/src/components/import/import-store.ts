"use client"

import { useSyncExternalStore } from "react"
import type { ImportJob } from "@/lib/types"

/** sessionStorage key of the running import, so a reload resumes polling. */
export const IMPORT_JOB_KEY = "assetku.import.job"

interface ImportState {
  /** The dialog is visible. */
  open: boolean
  /** Running job being polled (mirrored in sessionStorage). */
  jobId: string | null
  /** Finished job (done or failed) shown on the result step until the dialog closes. */
  result: ImportJob | null
}

const SERVER_STATE: ImportState = { open: false, jobId: null, result: null }
let state: ImportState | null = null
const listeners = new Set<() => void>()

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage
  } catch {
    return null // storage disabled
  }
}

function current(): ImportState {
  state ??= { open: false, jobId: storage()?.getItem(IMPORT_JOB_KEY) ?? null, result: null }
  return state
}

function set(patch: Partial<ImportState>) {
  state = { ...current(), ...patch }
  listeners.forEach((l) => l())
}

function subscribe(l: () => void) {
  listeners.add(l)
  return () => listeners.delete(l)
}

/**
 * One import dialog and poller lives in the app shell; the list page buttons
 * and the toast action drive it through these functions.
 */
export function useImportState(): ImportState {
  return useSyncExternalStore(subscribe, current, () => SERVER_STATE)
}

export function getImportState(): ImportState {
  return current()
}

export function openImportDialog() {
  set({ open: true })
}

/** Closing drops a shown result; a running job keeps polling in the background. */
export function closeImportDialog() {
  set({ open: false, result: null })
}

export function startImportJob(id: string) {
  storage()?.setItem(IMPORT_JOB_KEY, id)
  set({ jobId: id, result: null })
}

/** The job reached done/failed; it is shown only if the dialog is open. */
export function finishImportJob(job: ImportJob) {
  storage()?.removeItem(IMPORT_JOB_KEY)
  set({ jobId: null, result: current().open ? job : null })
}

/** Forget the job without a result (e.g. it no longer exists). */
export function dropImportJob() {
  storage()?.removeItem(IMPORT_JOB_KEY)
  set({ jobId: null })
}

/** Opens the dialog on a finished job's result (toast action). */
export function showImportResult(job: ImportJob) {
  set({ open: true, result: job })
}

/** Back to the upload step. */
export function clearImportResult() {
  set({ result: null })
}

/** Test helper: reinitialise from sessionStorage (cleared unless keepStorage). */
export function resetImportStore({ keepStorage = false } = {}) {
  if (!keepStorage) storage()?.removeItem(IMPORT_JOB_KEY)
  state = null
  listeners.forEach((l) => l())
}
