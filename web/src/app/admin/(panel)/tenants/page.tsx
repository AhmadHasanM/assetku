"use client"

import Link from "next/link"
import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { ChevronRight, Plus } from "lucide-react"
import { toast } from "sonner"
import { TenantDialog } from "@/components/admin/tenant-dialog"
import { ToggleActiveDialog } from "@/components/admin/toggle-active-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { ADMIN_TENANTS_QUERY_KEY, useAdminTenants, type AdminTenant } from "@/lib/admin"
import { api, ApiError } from "@/lib/api"
import { formatDate } from "@/lib/format"
import type { Locale } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"

export default function AdminTenantsPage() {
  const t = useTranslations("admin")
  const tc = useTranslations("common")
  const locale = useLocale() as Locale
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const list = useAdminTenants()
  const [adding, setAdding] = useState(false)
  const [toggling, setToggling] = useState<AdminTenant | null>(null)

  const create = useMutation({
    mutationFn: (values: { name: string }) => api<AdminTenant>("/api/admin/tenants", { method: "POST", json: values }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ADMIN_TENANTS_QUERY_KEY })
      setAdding(false)
      toast.success(t("tenants.created"))
    },
  })

  const toggle = useMutation({
    mutationFn: (tenant: AdminTenant) =>
      api<AdminTenant>(`/api/admin/tenants/${tenant.id}`, { method: "PATCH", json: { active: !tenant.active } }),
    onSuccess: (tenant) => {
      qc.invalidateQueries({ queryKey: ADMIN_TENANTS_QUERY_KEY })
      toast.success(t(tenant.active ? "tenants.activated" : "tenants.deactivated"))
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setToggling(null),
  })

  const submit = async (values: { name: string }) => {
    try {
      await create.mutateAsync(values)
    } catch (err) {
      // Validation errors are shown on the field by the dialog; the rest become toasts.
      if (!(err instanceof ApiError && err.code === "validation")) toast.error(errorMessage(err))
      throw err
    }
  }

  const items = list.data ?? []

  const status = (tenant: AdminTenant) => (
    <Badge variant={tenant.active ? "success" : "locked"}>{tenant.active ? t("active") : t("inactive")}</Badge>
  )
  const toggleButton = (tenant: AdminTenant) => (
    <Button variant={tenant.active ? "ghost" : "secondary"} size="sm" onClick={() => setToggling(tenant)}>
      {tenant.active ? t("tenants.deactivate") : t("tenants.activate")}
    </Button>
  )

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-[28px] sm:text-4xl">{t("tenants.title")}</h1>
        <Button onClick={() => setAdding(true)}>
          <Plus />
          {t("tenants.add")}
        </Button>
      </div>

      {list.isPending ? (
        <div className="flex flex-col gap-3" data-testid="tenants-loading">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-16 rounded-lg" />
          ))}
        </div>
      ) : list.isError ? (
        <Card className="items-center gap-4 py-10 text-center">
          <p role="alert">{t("tenants.loadFailed")}</p>
          <Button variant="secondary" onClick={() => list.refetch()}>
            {tc("retry")}
          </Button>
        </Card>
      ) : items.length === 0 ? (
        <Card className="items-center gap-2 py-12 text-center">
          <p className="font-heading text-xl font-semibold">{t("tenants.empty")}</p>
          <p className="text-muted-foreground">{t("tenants.emptyHint")}</p>
        </Card>
      ) : (
        <>
          <Card className="hidden gap-0 p-0 md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("tenants.name")}</TableHead>
                  <TableHead>{t("tenants.status")}</TableHead>
                  <TableHead className="text-right">{t("tenants.users")}</TableHead>
                  <TableHead className="text-right">{t("tenants.assets")}</TableHead>
                  <TableHead>{t("tenants.createdAt")}</TableHead>
                  <TableHead className="w-36 text-right">
                    <span className="sr-only">{t("actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((tenant) => (
                  <TableRow key={tenant.id}>
                    <TableCell className="max-w-72">
                      <Link
                        href={`/admin/tenants/${tenant.id}`}
                        className="inline-flex min-h-11 max-w-full items-center truncate font-semibold text-foreground underline-offset-4 outline-none hover:text-nebula-text hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                      >
                        <span className="truncate">{tenant.name}</span>
                      </Link>
                    </TableCell>
                    <TableCell>{status(tenant)}</TableCell>
                    <TableCell className="text-right">{tenant.user_count.toLocaleString(locale)}</TableCell>
                    <TableCell className="text-right">{tenant.asset_count.toLocaleString(locale)}</TableCell>
                    <TableCell className="text-muted-foreground">{formatDate(tenant.created_at, locale)}</TableCell>
                    <TableCell className="text-right">{toggleButton(tenant)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

          <ul className="flex flex-col gap-3 md:hidden">
            {items.map((tenant) => (
              <li key={tenant.id}>
                <Card className="gap-3 p-4">
                  <Link
                    href={`/admin/tenants/${tenant.id}`}
                    aria-label={t("tenants.open", { name: tenant.name })}
                    className="-m-1 flex min-h-11 items-start justify-between gap-2 rounded-md p-1 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    <span className="min-w-0">
                      <span className="block font-heading text-lg font-semibold break-words">{tenant.name}</span>
                      <span className="block text-sm text-muted-foreground">
                        {t("tenants.userCount", { count: tenant.user_count })} ·{" "}
                        {t("tenants.assetCount", { count: tenant.asset_count })}
                      </span>
                    </span>
                    <ChevronRight aria-hidden className="mt-1 size-5 shrink-0 text-subtle-foreground" />
                  </Link>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      {status(tenant)}
                      <span className="text-sm text-subtle-foreground">
                        {t("tenants.createdOn", { date: formatDate(tenant.created_at, locale) })}
                      </span>
                    </div>
                    {toggleButton(tenant)}
                  </div>
                </Card>
              </li>
            ))}
          </ul>
        </>
      )}

      <TenantDialog open={adding} onOpenChange={setAdding} tenant={null} onSubmit={submit} />

      <ToggleActiveDialog
        target={toggling}
        pending={toggle.isPending}
        title={(x) => t(x.active ? "tenants.deactivateTitle" : "tenants.activateTitle", { name: x.name })}
        description={(x) => t(x.active ? "tenants.deactivateDescription" : "tenants.activateDescription")}
        confirmLabel={(x) => t(x.active ? "tenants.deactivate" : "tenants.activate")}
        onCancel={() => setToggling(null)}
        onConfirm={(x) => toggle.mutate(x)}
      />
    </div>
  )
}
