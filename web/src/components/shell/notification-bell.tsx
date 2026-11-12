"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Bell, CheckCheck } from "lucide-react"
import { useLocale, useTranslations } from "next-intl"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { formatRelative } from "@/lib/format"
import type { Locale, NotificationItem, NotificationList } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import { cn } from "@/lib/utils"

const QUERY_KEY = ["notifications"]
const POLL_MS = 60_000

/** Badge text: the count, capped at "99+". */
export function badgeText(unread: number): string {
  return unread > 99 ? "99+" : String(unread)
}

/** Header bell: unread badge and a popover with the last 20 notifications. */
export function NotificationBell() {
  const t = useTranslations("notifications")
  const locale = useLocale() as Locale
  const router = useRouter()
  const qc = useQueryClient()
  const errorMessage = useErrorMessage()
  const [open, setOpen] = useState(false)

  const query = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => api<NotificationList>("/api/notifications"),
    refetchInterval: POLL_MS,
    refetchOnWindowFocus: "always",
  })
  const unread = query.data?.unread ?? 0
  const items = query.data?.items ?? []

  const markRead = useMutation({
    mutationFn: (n: NotificationItem) => api(`/api/notifications/${n.id}/read`, { method: "POST" }),
  })
  const markAll = useMutation({
    mutationFn: () => api("/api/notifications/read-all", { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QUERY_KEY }),
    onError: (err) => toast.error(errorMessage(err)),
  })

  const openItem = async (n: NotificationItem) => {
    if (!n.read_at) {
      try {
        await markRead.mutateAsync(n)
      } catch (err) {
        // Opening the asset matters more than the read flag; report and carry on.
        toast.error(errorMessage(err))
      }
      qc.invalidateQueries({ queryKey: QUERY_KEY })
    }
    setOpen(false)
    if (n.asset_id) router.push(`/assets/${n.asset_id}`)
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-11"
          aria-label={unread > 0 ? t("openUnread", { count: unread }) : t("open")}
        >
          <Bell aria-hidden className="size-5" />
          {unread > 0 && (
            <span
              aria-hidden
              className="absolute -top-0.5 -right-0.5 flex h-5 min-w-5 items-center justify-center rounded-full bg-error px-1 text-xs leading-none font-bold text-background tabular-nums"
            >
              {badgeText(unread)}
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[min(24rem,calc(100vw-1.5rem))] gap-0 p-0">
        <div className="flex items-center justify-between gap-2 border-b px-4 py-2">
          <h2 className="font-heading text-lg font-semibold">{t("title")}</h2>
          <Button
            variant="ghost"
            size="sm"
            className="h-11"
            disabled={unread === 0 || markAll.isPending}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck aria-hidden />
            {t("markAllRead")}
          </Button>
        </div>
        <div className="max-h-[min(28rem,70dvh)] overflow-y-auto p-1.5">
          {query.isPending ? (
            <div className="flex flex-col gap-2 p-2">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-14 rounded-md" />
              ))}
            </div>
          ) : query.isError ? (
            <div className="flex flex-col items-center gap-3 p-4 text-center">
              <p role="alert" className="text-sm">
                {t("loadFailed")}
              </p>
            </div>
          ) : items.length === 0 ? (
            <p className="p-6 text-center text-sm text-muted-foreground">{t("empty")}</p>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {items.map((n) => (
                <li key={n.id}>
                  <button
                    type="button"
                    onClick={() => openItem(n)}
                    className={cn(
                      "flex min-h-11 w-full items-start gap-3 rounded-md px-3 py-2.5 text-left outline-none transition-colors",
                      "hover:bg-nebula/10 focus-visible:ring-3 focus-visible:ring-ring/50",
                    )}
                  >
                    <span
                      className={cn("mt-1.5 size-2 shrink-0 rounded-full", n.read_at ? "bg-transparent" : "bg-nebula")}
                    >
                      {!n.read_at && <span className="sr-only">{t("unread")}</span>}
                    </span>
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className={cn("text-sm", n.read_at ? "font-medium" : "font-semibold text-foreground")}>
                        {n.title}
                      </span>
                      <span className="text-sm text-muted-foreground">{n.body}</span>
                      <span className="text-xs text-subtle-foreground">{formatRelative(n.created_at, locale)}</span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
