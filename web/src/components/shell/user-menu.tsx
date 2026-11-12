"use client"

import Link from "next/link"
import { useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { ChevronDown, KeyRound, Languages, LogOut, Moon, Sun, UserRound } from "lucide-react"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { api } from "@/lib/api"
import type { Locale, Me, Theme } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { useChangePreferences } from "./use-preferences"

function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join("") || "?"
  )
}

export function UserMenu({ me }: { me: Me }) {
  const t = useTranslations("userMenu")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const prefs = useChangePreferences()
  const [loggingOut, setLoggingOut] = useState(false)

  const logout = async () => {
    setLoggingOut(true)
    try {
      await api("/api/auth/logout", { method: "POST" })
      qc.clear()
      // Full page load so no client state of the previous user survives.
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.assign("/login")
    } catch (err) {
      setLoggingOut(false)
      toast.error(errorMessage(err))
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="h-11 gap-2 px-1.5 sm:pr-3" aria-label={t("open")}>
          <span
            aria-hidden
            className="flex size-9 items-center justify-center rounded-full bg-nebula/20 font-heading text-sm font-semibold text-nebula-text"
          >
            {initials(me.name)}
          </span>
          <span className="hidden max-w-40 truncate text-sm font-semibold text-foreground sm:inline">{me.name}</span>
          <ChevronDown aria-hidden className="hidden size-4 sm:block" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel className="flex flex-col gap-0.5 py-2">
          <span className="truncate text-sm font-semibold text-foreground">{me.name}</span>
          <span className="truncate text-xs font-normal">{me.email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link href="/profile">
            <UserRound />
            {t("profile")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/profile#password">
            <KeyRound />
            {t("changePassword")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Languages />
            {t("language")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-48">
            <DropdownMenuRadioGroup value={me.locale} onValueChange={(v) => prefs.setLocale(v as Locale)}>
              <DropdownMenuRadioItem value="id">{t("indonesian")}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="en">{t("english")}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            {me.theme === "dark" ? <Moon /> : <Sun />}
            {t("theme")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-48">
            <DropdownMenuRadioGroup value={me.theme} onValueChange={(v) => prefs.setTheme(v as Theme)}>
              <DropdownMenuRadioItem value="dark">
                <Moon />
                {t("dark")}
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="light">
                <Sun />
                {t("light")}
              </DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={logout} disabled={loggingOut}>
          <LogOut />
          {t("logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
