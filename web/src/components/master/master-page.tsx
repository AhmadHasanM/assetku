"use client"

import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { Pencil, Plus, Search, Trash2 } from "lucide-react"
import { toast } from "sonner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, ApiError } from "@/lib/api"
import { invalidateMasterData } from "@/lib/invalidate"
import { useErrorMessage } from "@/lib/use-error-message"
import { KIND_CONFIG, type MasterItem, type MasterKind } from "./kinds"
import { MasterFormDialog, type MasterValues } from "./master-form-dialog"

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return v
}

/** List + add/edit/delete page for one master data kind. */
export function MasterPage({ kind }: { kind: MasterKind }) {
  const t = useTranslations("master")
  const tc = useTranslations("common")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const config = KIND_CONFIG[kind]

  const [search, setSearch] = useState("")
  const q = useDebounced(search.trim(), 250)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<MasterItem | null>(null)
  const [deleting, setDeleting] = useState<MasterItem | null>(null)

  const list = useQuery({
    queryKey: [kind, q],
    queryFn: () => api<MasterItem[]>(`/api/${kind}${q ? `?q=${encodeURIComponent(q)}` : ""}`),
    placeholderData: (prev) => prev,
  })

  const save = useMutation({
    mutationFn: (values: Partial<MasterValues>) =>
      editing
        ? api<MasterItem>(`/api/${kind}/${editing.id}`, { method: "PATCH", json: values })
        : api<MasterItem>(`/api/${kind}`, { method: "POST", json: values }),
    onSuccess: () => {
      invalidateMasterData(qc, kind)
      setFormOpen(false)
      toast.success(t("saved"))
    },
  })

  const remove = useMutation({
    mutationFn: (item: MasterItem) => api<void>(`/api/${kind}/${item.id}`, { method: "DELETE" }),
    onSuccess: () => {
      invalidateMasterData(qc, kind)
      toast.success(t("deleted"))
    },
    onError: (err) => {
      // A row referenced only by past records reports count 0: avoid "used by 0 assets".
      if (err instanceof ApiError && err.code === "in_use" && Number(err.fields.count) === 0) {
        toast.error(t("inUseGeneric"))
      } else {
        toast.error(errorMessage(err))
      }
    },
    onSettled: () => setDeleting(null),
  })

  const submit = async (values: Partial<MasterValues>) => {
    try {
      await save.mutateAsync(values)
    } catch (err) {
      // Validation errors are shown on the fields by the dialog; the rest become toasts.
      if (!(err instanceof ApiError && err.code === "validation")) toast.error(errorMessage(err))
      throw err
    }
  }

  const openAdd = () => {
    setEditing(null)
    setFormOpen(true)
  }
  const openEdit = (item: MasterItem) => {
    setEditing(item)
    setFormOpen(true)
  }

  const items = list.data ?? []
  const title = t(`kinds.${kind}.title`)

  const actions = (item: MasterItem) => (
    <div className="flex justify-end gap-1">
      <Button variant="ghost" size="icon" aria-label={`${t("edit")} ${item.name}`} onClick={() => openEdit(item)}>
        <Pencil />
      </Button>
      <Button variant="ghost" size="icon" aria-label={`${t("delete")} ${item.name}`} onClick={() => setDeleting(item)}>
        <Trash2 />
      </Button>
    </div>
  )

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-[28px] sm:text-4xl">{title}</h1>
        <Button onClick={openAdd}>
          <Plus />
          {t("add")}
        </Button>
      </div>

      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-subtle-foreground" aria-hidden />
        <Input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("searchPlaceholder")}
          aria-label={t("searchPlaceholder")}
          className="pl-12"
        />
      </div>

      {list.isPending ? (
        <div className="flex flex-col gap-3" data-testid="master-loading">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-16 rounded-lg" />
          ))}
        </div>
      ) : list.isError ? (
        <Card className="items-center gap-4 py-10 text-center">
          <p role="alert">{t("loadFailed")}</p>
          <Button variant="secondary" onClick={() => list.refetch()}>
            {tc("retry")}
          </Button>
        </Card>
      ) : items.length === 0 ? (
        <Card className="items-center gap-2 py-12 text-center">
          <p className="font-heading text-xl font-semibold">{q ? t("emptySearch") : t("emptyTitle")}</p>
          {!q && <p className="text-muted-foreground">{t("emptyHint")}</p>}
        </Card>
      ) : (
        <>
          <Card className="hidden gap-0 p-0 md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("name")}</TableHead>
                  {config.columns.map((c) => (
                    <TableHead key={c}>{t(`fields.${c}`)}</TableHead>
                  ))}
                  <TableHead className="text-right">{t("assetCount")}</TableHead>
                  <TableHead className="w-28 text-right">
                    <span className="sr-only">{t("actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-semibold">{item.name}</TableCell>
                    {config.columns.map((c) => (
                      <TableCell key={c} className="max-w-56 truncate text-muted-foreground">
                        {item[c as keyof MasterItem] || "-"}
                      </TableCell>
                    ))}
                    <TableCell className="text-right">{item.asset_count}</TableCell>
                    <TableCell>{actions(item)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

          <ul className="flex flex-col gap-3 md:hidden">
            {items.map((item) => (
              <li key={item.id}>
                <Card className="gap-3 p-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="font-heading text-lg font-semibold break-words">{item.name}</p>
                      <p className="text-sm text-muted-foreground">{t("assets", { count: item.asset_count })}</p>
                    </div>
                    {actions(item)}
                  </div>
                  {config.fields
                    .filter((f) => f !== "name" && item[f])
                    .map((f) => (
                      <p key={f} className="text-sm break-words">
                        <span className="text-subtle-foreground">{t(`fields.${f}`)}: </span>
                        {item[f]}
                      </p>
                    ))}
                </Card>
              </li>
            ))}
          </ul>
        </>
      )}

      <MasterFormDialog kind={kind} open={formOpen} onOpenChange={setFormOpen} item={editing} onSubmit={submit} />

      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && !remove.isPending && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDescription", { name: deleting?.name ?? "" })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={remove.isPending}
              onClick={(e) => {
                e.preventDefault()
                if (deleting) remove.mutate(deleting)
              }}
            >
              {remove.isPending ? t("deleting") : t("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
