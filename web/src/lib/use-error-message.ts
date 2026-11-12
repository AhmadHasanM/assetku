"use client"

import { useTranslations } from "next-intl"
import { ApiError } from "./api"

/** Translates an API error into UI text via errors.<code> (falls back to errors.generic). */
export function useErrorMessage() {
  const t = useTranslations("errors")
  return (err: unknown): string => {
    if (err instanceof ApiError) {
      const key = err.code
      if (t.has(key)) return t(key, { count: err.fields.count ?? "" })
    }
    return t("generic")
  }
}

/** Translates a field error code (e.g. "too_short") via fieldErrors.<code>. */
export function useFieldErrorMessage() {
  const t = useTranslations("fieldErrors")
  return (code: string | undefined): string | undefined => {
    if (!code) return undefined
    return t.has(code) ? t(code) : t("invalid")
  }
}
