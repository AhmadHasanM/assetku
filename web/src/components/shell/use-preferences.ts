"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useTheme } from "next-themes"
import { useTranslations } from "next-intl"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { readCookie } from "@/lib/api"
import { ME_QUERY_KEY, useUpdateMe } from "@/lib/me"
import { writeLocaleCookie } from "@/lib/preferences"
import { LOCALE_COOKIE } from "@/i18n/locale"
import type { Locale, Me, Theme } from "@/lib/types"

/** Applies Me.locale (cookie + refresh) and Me.theme (next-themes) when they differ from the UI. */
export function useApplyPreferences(me: Me | undefined) {
  const router = useRouter()
  const { setTheme } = useTheme()
  const locale = me?.locale
  const theme = me?.theme
  useEffect(() => {
    if (!locale || !theme) return
    setTheme(theme)
    if (readCookie(LOCALE_COOKIE) !== locale) {
      writeLocaleCookie(locale)
      router.refresh()
    }
  }, [locale, theme, router, setTheme])
}

/** Changes language/theme immediately and saves it to the profile (PATCH /api/me). */
export function useChangePreferences() {
  const t = useTranslations("profile")
  const qc = useQueryClient()
  const updateMe = useUpdateMe()

  const save = (patch: { locale: Locale } | { theme: Theme }) => {
    const previous = qc.getQueryData<Me>(ME_QUERY_KEY)
    if (previous) qc.setQueryData<Me>(ME_QUERY_KEY, { ...previous, ...patch })
    updateMe.mutate(patch, {
      onError: () => {
        if (previous) qc.setQueryData(ME_QUERY_KEY, previous)
        toast.error(t("preferencesSaveFailed"))
      },
    })
  }

  return {
    // The optimistic cache update triggers useApplyPreferences, which writes
    // the cookie / theme and refreshes server-rendered messages.
    setLocale: (locale: Locale) => save({ locale }),
    setTheme: (theme: Theme) => save({ theme }),
  }
}
