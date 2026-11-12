import { DEFAULT_LOCALE, LOCALES, type Locale } from "@/lib/types"

/** Cookie holding the UI language; written by the client from Me.locale. */
export const LOCALE_COOKIE = "locale"

export function toLocale(value: string | null | undefined): Locale {
  return LOCALES.includes(value as Locale) ? (value as Locale) : DEFAULT_LOCALE
}
