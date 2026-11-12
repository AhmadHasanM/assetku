import { LOCALE_COOKIE } from "@/i18n/locale"
import type { Locale } from "./types"

/** Persists the UI language for server rendering (read by i18n/request.ts). */
export function writeLocaleCookie(locale: Locale) {
  document.cookie = `${LOCALE_COOKIE}=${locale}; Path=/; Max-Age=${365 * 24 * 3600}; SameSite=Lax`
}
