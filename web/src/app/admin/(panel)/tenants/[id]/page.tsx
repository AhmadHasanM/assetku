"use client"

import Link from "next/link"
import { useState } from "react"
import { useParams } from "next/navigation"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useLocale, useTranslations } from "next-intl"
import { ArrowLeft, KeyRound, MoreHorizontal, Pencil, Plus, Power, PowerOff, SearchX } from "lucide-react"
import { toast } from "sonner"
import { ResetPasswordDialog } from "@/components/admin/reset-password-dialog"
import { TenantDialog } from "@/components/admin/tenant-dialog"
import { ToggleActiveDialog } from "@/components/admin/toggle-active-dialog"
import { UserDialog, type UserValues } from "@/components/admin/user-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  ADMIN_TENANTS_QUERY_KEY,
  adminUsersQueryKey,
  useAdminTenants,
  useAdminUsers,
  type AdminTenant,
  type AdminUser,
} from "@/lib/admin"
import { api, ApiError } from "@/lib/api"
import { formatDate } from "@/lib/format"
import type { Locale } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"

/** Shows validation errors inline (the dialog does it) and everything else as a toast. */
function useSubmitWithToast() {
  const errorMessage = useErrorMessage()
  return async (run: () => Promise<unknown>) => {
    try {
      await run()
    } catch (err) {
      if (!(err instanceof ApiError && err.code === "validation")) toast.error(errorMessage(err))
      throw err
    }
  }
}

export default function AdminTenantPage() {
  const t = useTranslations("admin")
  const tc = useTranslations("common")
  const locale = useLocale() as Locale
  const errorMessage = useErrorMessage()
  const withToast = useSubmitWithToast()
  const qc = useQueryClient()
  const { id } = useParams<{ id: string }>()

  // There is no single-tenant endpoint; the list is small and already cached from /admin/tenants.
  const tenants = useAdminTenants()
  const tenant = tenants.data?.find((x) => x.id === id)
  const users = useAdminUsers(id)

  const [renaming, setRenaming] = useState(false)
  const [userForm, setUserForm] = useState<{ open: boolean; user: AdminUser | null }>({ open: false, user: null })
  const [toggling, setToggling] = useState<AdminUser | null>(null)
  const [resetting, setResetting] = useState<AdminUser | null>(null)

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: adminUsersQueryKey(id) })
    qc.invalidateQueries({ queryKey: ADMIN_TENANTS_QUERY_KEY, exact: true })
  }

  const rename = useMutation({
    mutationFn: (values: { name: string }) =>
      api<AdminTenant>(`/api/admin/tenants/${id}`, { method: "PATCH", json: values }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ADMIN_TENANTS_QUERY_KEY, exact: true })
      setRenaming(false)
      toast.success(t("tenants.saved"))
    },
  })

  const saveUser = useMutation({
    mutationFn: ({ user, values }: { user: AdminUser | null; values: Partial<UserValues> }) =>
      user
        ? api<AdminUser>(`/api/admin/users/${user.id}`, { method: "PATCH", json: values })
        : api<AdminUser>(`/api/admin/tenants/${id}/users`, { method: "POST", json: values }),
    onSuccess: (_, { user }) => {
      invalidate()
      setUserForm((f) => ({ ...f, open: false }))
      toast.success(t(user ? "users.saved" : "users.created"))
    },
  })

  const toggleUser = useMutation({
    mutationFn: (user: AdminUser) =>
      api<AdminUser>(`/api/admin/users/${user.id}`, { method: "PATCH", json: { active: !user.active } }),
    onSuccess: (user) => {
      invalidate()
      toast.success(t(user.active ? "users.activated" : "users.deactivated"))
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setToggling(null),
  })

  const resetPassword = useMutation({
    mutationFn: ({ user, password }: { user: AdminUser; password: string }) =>
      api<void>(`/api/admin/users/${user.id}/reset-password`, { method: "POST", json: { password } }),
    onSuccess: () => {
      setResetting(null)
      toast.success(t("users.resetDone"))
    },
  })

  const notFound =
    (users.error instanceof ApiError && users.error.status === 404) || (tenants.isSuccess && !tenant)
  if (notFound) {
    return (
      <Card className="mx-auto w-full max-w-md items-center gap-4 py-12 text-center">
        <SearchX aria-hidden className="size-10 text-subtle-foreground" />
        <p className="font-heading text-xl font-semibold">{t("tenants.notFound")}</p>
        <Button variant="secondary" asChild>
          <Link href="/admin/tenants">{t("tenants.back")}</Link>
        </Button>
      </Card>
    )
  }

  const items = users.data ?? []

  const status = (user: AdminUser) => (
    <Badge variant={user.active ? "success" : "locked"}>{user.active ? t("active") : t("inactive")}</Badge>
  )

  const actions = (user: AdminUser) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("users.rowActions", { name: user.name })}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => setUserForm({ open: true, user })}>
          <Pencil />
          {t("users.edit")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => setResetting(user)}>
          <KeyRound />
          {t("users.resetPassword")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {user.active ? (
          <DropdownMenuItem variant="destructive" onSelect={() => setToggling(user)}>
            <PowerOff />
            {t("users.deactivate")}
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onSelect={() => setToggling(user)}>
            <Power />
            {t("users.activate")}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <Button variant="ghost" asChild className="-ml-3 self-start">
        <Link href="/admin/tenants">
          <ArrowLeft />
          {t("tenants.back")}
        </Link>
      </Button>

      <div className="flex flex-wrap items-start justify-between gap-3">
        {tenant ? (
          <div className="flex min-w-0 flex-col gap-2">
            <h1 className="text-[28px] break-words sm:text-4xl">{tenant.name}</h1>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={tenant.active ? "success" : "locked"}>
                {tenant.active ? t("active") : t("inactive")}
              </Badge>
              <span className="text-sm text-muted-foreground">
                {t("tenants.assetCount", { count: tenant.asset_count })} ·{" "}
                {t("tenants.createdOn", { date: formatDate(tenant.created_at, locale) })}
              </span>
            </div>
          </div>
        ) : (
          <Skeleton className="h-16 w-64 rounded-lg" />
        )}
        <Button variant="secondary" onClick={() => setRenaming(true)} disabled={!tenant}>
          <Pencil />
          {t("tenants.rename")}
        </Button>
      </div>

      <section className="flex flex-col gap-4" aria-labelledby="users-title">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="users-title" className="text-[22px] sm:text-[28px]">
            {t("users.title")}
          </h2>
          <Button onClick={() => setUserForm({ open: true, user: null })}>
            <Plus />
            {t("users.add")}
          </Button>
        </div>

        {users.isPending ? (
          <div className="flex flex-col gap-3" data-testid="users-loading">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </div>
        ) : users.isError ? (
          <Card className="items-center gap-4 py-10 text-center">
            <p role="alert">{t("users.loadFailed")}</p>
            <Button variant="secondary" onClick={() => users.refetch()}>
              {tc("retry")}
            </Button>
          </Card>
        ) : items.length === 0 ? (
          <Card className="items-center gap-2 py-12 text-center">
            <p className="font-heading text-xl font-semibold">{t("users.empty")}</p>
            <p className="text-muted-foreground">{t("users.emptyHint")}</p>
          </Card>
        ) : (
          <>
            <Card className="hidden gap-0 p-0 md:block">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("users.name")}</TableHead>
                    <TableHead>{t("users.email")}</TableHead>
                    <TableHead>{t("users.position")}</TableHead>
                    <TableHead>{t("users.status")}</TableHead>
                    <TableHead className="w-16 text-right">
                      <span className="sr-only">{t("actions")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((user) => (
                    <TableRow key={user.id}>
                      <TableCell className="max-w-56 truncate font-semibold">{user.name}</TableCell>
                      <TableCell className="max-w-64 truncate text-muted-foreground">{user.email}</TableCell>
                      <TableCell className="max-w-48 truncate text-muted-foreground">{user.position || "-"}</TableCell>
                      <TableCell>{status(user)}</TableCell>
                      <TableCell className="text-right">{actions(user)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>

            <ul className="flex flex-col gap-3 md:hidden">
              {items.map((user) => (
                <li key={user.id}>
                  <Card className="gap-2 p-4">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="font-heading text-lg font-semibold break-words">{user.name}</p>
                        <p className="text-sm break-all text-muted-foreground">{user.email}</p>
                      </div>
                      {actions(user)}
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      {status(user)}
                      {user.position && <span className="text-sm text-subtle-foreground">{user.position}</span>}
                    </div>
                  </Card>
                </li>
              ))}
            </ul>
          </>
        )}
      </section>

      <TenantDialog
        open={renaming}
        onOpenChange={setRenaming}
        tenant={tenant ?? null}
        onSubmit={(values) => withToast(() => rename.mutateAsync(values))}
      />
      <UserDialog
        open={userForm.open}
        onOpenChange={(open) => setUserForm((f) => ({ ...f, open }))}
        user={userForm.user}
        onSubmit={(values) => withToast(() => saveUser.mutateAsync({ user: userForm.user, values }))}
      />
      <ResetPasswordDialog
        user={resetting}
        onOpenChange={(open) => !open && setResetting(null)}
        onSubmit={(password) => withToast(() => resetPassword.mutateAsync({ user: resetting!, password }))}
      />
      <ToggleActiveDialog
        target={toggling}
        pending={toggleUser.isPending}
        title={(u) => t(u.active ? "users.deactivateTitle" : "users.activateTitle", { name: u.name })}
        description={(u) => t(u.active ? "users.deactivateDescription" : "users.activateDescription")}
        confirmLabel={(u) => t(u.active ? "users.deactivate" : "users.activate")}
        onCancel={() => setToggling(null)}
        onConfirm={(u) => toggleUser.mutate(u)}
      />
    </div>
  )
}
