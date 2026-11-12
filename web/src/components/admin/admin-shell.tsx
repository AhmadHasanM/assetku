"use client"

import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { useTheme } from "next-themes"
import { Building2, ChevronDown, Languages, LogOut, Moon, ShieldCheck, Sun } from "lucide-react"
import { toast } from "sonner"
import { Logo } from "@/components/shell/logo"
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
import { Skeleton } from "@/components/ui/skeleton"
import { useAdminMe, type AdminMe } from "@/lib/admin"
import { api } from "@/lib/api"
import { PRODUCT_NAME } from "@/lib/brand"
import { writeLocaleCookie } from "@/lib/preferences"
import type { Locale } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { cn } from "@/lib/utils"

/**
 * Super admin menu. Admins have no saved preferences: language lives in the
 * locale cookie and theme in next-themes' local storage, same as before login.
 */
function AdminMenu({ me }: { me: AdminMe }) {
  const t = useTranslations("userMenu")
  const ta = useTranslations("admin")
  const errorMessage = useErrorMessage()
  const locale = useLocale()
  const router = useRouter()
  const qc = useQueryClient()
  const { resolvedTheme, setTheme } = useTheme()
  const theme = resolvedTheme === "light" ? "light" : "dark"
  const [loggingOut, setLoggingOut] = useState(false)

  const setLocale = (l: Locale) => {
    writeLocaleCookie(l)
    router.refresh()
  }

  const logout = async () => {
    setLoggingOut(true)
    try {
      await api("/api/admin/auth/logout", { method: "POST" })
      qc.clear()
      // Full page load so no admin state survives.
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.assign("/admin/login")
    } catch (err) {
      setLoggingOut(false)
      toast.error(errorMessage(err))
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="h-11 gap-2 px-1.5 sm:pr-3" aria-label={ta("menu")}>
          <span
            aria-hidden
            className="flex size-9 items-center justify-center rounded-full bg-nebula/20 text-nebula-text"
          >
            <ShieldCheck className="size-5" />
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
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Languages />
            {t("language")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-48">
            <DropdownMenuRadioGroup value={locale} onValueChange={(v) => setLocale(v as Locale)}>
              <DropdownMenuRadioItem value="id">{t("indonesian")}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="en">{t("english")}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            {theme === "dark" ? <Moon /> : <Sun />}
            {t("theme")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-48">
            <DropdownMenuRadioGroup value={theme} onValueChange={setTheme}>
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

/** Super admin layout: own header ("<product> Admin", Tenants menu, admin menu) and page content. */
export function AdminShell({ children }: { children: React.ReactNode }) {
  const t = useTranslations("admin")
  const pathname = usePathname()
  const { data: me } = useAdminMe()
  const onTenants = pathname.startsWith("/admin/tenants")

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="sticky top-0 z-30 flex h-16 items-center gap-2 border-b bg-background/90 px-3 backdrop-blur sm:gap-6 sm:px-6">
        <Link
          href="/admin/tenants"
          aria-label={t("home", { product: PRODUCT_NAME })}
          className="flex min-h-11 min-w-0 items-center rounded-md px-1 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <Logo label={`${PRODUCT_NAME} Admin`} className="truncate text-lg sm:text-xl" />
        </Link>
        <nav aria-label={t("mainMenu")} className="hidden sm:block">
          <Link
            href="/admin/tenants"
            aria-current={onTenants ? "page" : undefined}
            className={cn(
              "flex min-h-11 items-center gap-2 rounded-full px-4 text-sm font-semibold text-muted-foreground outline-none hover:bg-nebula/10 hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
              onTenants && "bg-nebula/15 text-nebula-text",
            )}
          >
            <Building2 className="size-5" aria-hidden />
            {t("tenants.title")}
          </Link>
        </nav>
        <div className="ml-auto flex items-center">
          {me ? <AdminMenu me={me} /> : <Skeleton className="h-11 w-11 rounded-full sm:w-36" />}
        </div>
      </header>
      <main className="flex min-w-0 flex-1 flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">{children}</main>
    </div>
  )
}
