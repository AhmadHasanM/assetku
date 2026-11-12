"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { cn } from "@/lib/utils"
import { NAV_ITEMS, isActive } from "./nav-items"

/** Sidebar menu links, shared by the desktop sidebar and the mobile Sheet. */
export function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const t = useTranslations("nav")
  const pathname = usePathname()
  return (
    <nav aria-label={t("mainMenu")} className="flex flex-col gap-1">
      {NAV_ITEMS.map(({ href, labelKey, icon: Icon }) => {
        const active = isActive(pathname, href)
        return (
          <Link
            key={href}
            href={href}
            onClick={onNavigate}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex min-h-11 items-center gap-3 rounded-md px-3.5 py-2.5 text-sm text-sidebar-foreground transition-colors outline-none hover:bg-nebula/8 hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
              active && "bg-sidebar-accent font-semibold text-sidebar-accent-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            )}
          >
            <Icon aria-hidden className="size-[18px] shrink-0" />
            {t(labelKey)}
          </Link>
        )
      })}
    </nav>
  )
}
