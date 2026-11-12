"use client"

import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { ChevronsUpDown, X } from "lucide-react"
import { toast } from "sonner"
import { cn } from "cn"
import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { api } from "@/lib/api"
import type { MasterRef } from "@/lib/types"
import { useErrorMessage } from "@/lib/use-error-message"
import type { MasterItem, MasterKind } from "@/components/master/kinds"

const SEARCH_DEBOUNCE_MS = 200

/** Makes a ghost Button look like an Input (48px, radius 12px, 2px border), for popover triggers. */
export const INPUT_TRIGGER_CLASS = cn(
  "h-12 w-full justify-between rounded-md border-2 border-border bg-input px-4 text-base font-normal text-foreground",
  "hover:border-nebula-text hover:bg-input hover:text-foreground focus-visible:border-nebula-text focus-visible:ring-0 focus-visible:glow-nebula-sm",
  "aria-expanded:bg-input aria-expanded:text-foreground aria-invalid:border-error aria-invalid:ring-0 aria-invalid:glow-error-sm",
)
export const PLACEHOLDER_CLASS = "text-subtle-foreground hover:text-subtle-foreground aria-expanded:text-subtle-foreground"

interface Props {
  kind: MasterKind
  value: MasterRef | null
  onChange: (value: MasterRef | null) => void
  id?: string
  invalid?: boolean
  /** Shown when nothing is selected. Label the trigger with a <label htmlFor={id}>. */
  placeholder: string
  onBlur?: () => void
}

/**
 * Searchable picker for a master data row (GET /api/{kind}?q=). The last item
 * adds the typed name (POST /api/{kind}) and selects it right away.
 */
export function MasterCombobox({ kind, value, onChange, id, invalid, placeholder, onBlur }: Props) {
  const t = useTranslations("assetForm.combobox")
  const errorMessage = useErrorMessage()
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const [q, setQ] = useState("")
  useEffect(() => {
    const timer = setTimeout(() => setQ(search.trim()), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [search])

  // Same key shape as the master data pages, so their cache is shared and invalidated together.
  const list = useQuery({
    queryKey: [kind, q],
    queryFn: () => api<MasterItem[]>(`/api/${kind}${q ? `?q=${encodeURIComponent(q)}` : ""}`),
    enabled: open,
    placeholderData: (prev) => prev,
  })

  const create = useMutation({
    mutationFn: (name: string) => api<MasterItem>(`/api/${kind}`, { method: "POST", json: { name } }),
    onSuccess: (item) => {
      qc.invalidateQueries({ queryKey: [kind] })
      toast.success(t("created", { name: item.name }))
      select({ id: item.id, name: item.name })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const select = (next: MasterRef | null) => {
    onChange(next)
    setOpen(false)
    setSearch("")
    setQ("")
  }

  const typed = search.trim()
  const items = list.data ?? []
  // Names are unique per tenant regardless of case; offer "add" only for a new name.
  const canCreate = typed !== "" && !items.some((i) => i.name.toLowerCase() === typed.toLowerCase())

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) onBlur?.()
      }}
    >
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="ghost"
          role="combobox"
          aria-expanded={open}
          aria-invalid={invalid || undefined}
          className={cn(INPUT_TRIGGER_CLASS, !value && PLACEHOLDER_CLASS)}
        >
          <span className="truncate">{value?.name ?? placeholder}</span>
          <ChevronsUpDown className="text-muted-foreground" aria-hidden />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-(--radix-popover-trigger-width) min-w-64 p-1">
        <Command shouldFilter={false}>
          <CommandInput placeholder={t("search")} value={search} onValueChange={setSearch} maxLength={200} />
          <CommandList>
            {!canCreate && (
              <CommandEmpty>{list.isFetching && !list.data ? t("loading") : t("empty")}</CommandEmpty>
            )}
            <CommandGroup>
              {value && (
                <CommandItem value="__none__" onSelect={() => select(null)}>
                  <X className="text-muted-foreground" aria-hidden />
                  {t("none")}
                </CommandItem>
              )}
              {items.map((item) => (
                <CommandItem
                  key={item.id}
                  value={item.id}
                  data-checked={item.id === value?.id}
                  onSelect={() => select({ id: item.id, name: item.name })}
                >
                  <span className="truncate">{item.name}</span>
                </CommandItem>
              ))}
              {canCreate && (
                <CommandItem
                  value="__create__"
                  disabled={create.isPending}
                  onSelect={() => create.mutate(typed)}
                  className="text-nebula-text"
                >
                  <span className="truncate">{t("create", { name: typed })}</span>
                </CommandItem>
              )}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
