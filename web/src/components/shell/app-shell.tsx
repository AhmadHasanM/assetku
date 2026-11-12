"use client"

import { ImportDialog } from "@/components/import/import-dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { useMe } from "@/lib/me"
import { MobileNav } from "./mobile-nav"
import { Logo } from "./logo"
import { NotificationBell } from "./notification-bell"
import { Sidebar } from "./sidebar"
import { UserMenu } from "./user-menu"
import { useApplyPreferences } from "./use-preferences"

/** Authenticated layout: sidebar (desktop) or Sheet (mobile), header with user menu, page content. */
export function AppShell({ children }: { children: React.ReactNode }) {
  const { data: me } = useMe()
  useApplyPreferences(me)

  return (
    <div className="flex min-h-dvh">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-2 border-b bg-background/90 px-3 backdrop-blur sm:px-6">
          <MobileNav />
          <div className="lg:hidden">
            <Logo className="text-lg" />
          </div>
          <div className="ml-auto flex items-center gap-1 sm:gap-2">
            <NotificationBell />
            {me ? <UserMenu me={me} /> : <Skeleton className="h-11 w-11 rounded-full sm:w-36" />}
          </div>
        </header>
        <main className="flex min-w-0 flex-1 flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">{children}</main>
      </div>
      {/* Shell-level so an import keeps polling (and toasts) on any page. */}
      <ImportDialog />
    </div>
  )
}
