"use client"

import { useEffect, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { ChevronDown, RotateCcw, Search } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { api } from "@/lib/api"
import type { AssetFilters } from "@/lib/filters"
import { ASSET_CATEGORIES, ASSET_STATUSES } from "@/lib/types"
import type { MasterItem } from "@/components/master/kinds"

const ALL = "__all__"
const SEARCH_DEBOUNCE_MS = 300

interface MultiProps {
  label: string
  options: readonly { value: string; label: string }[]
  selected: string[]
  onChange: (next: string[]) => void
}

/** Popover + Command list with a checkbox per option. */
function MultiSelect({ label, options, selected, onChange }: MultiProps) {
  const t = useTranslations("assets.filters")
  const toggle = (v: string) => onChange(selected.includes(v) ? selected.filter((s) => s !== v) : [...selected, v])
  const summary =
    selected.length === 0
      ? label
      : selected.length === 1
        ? (options.find((o) => o.value === selected[0])?.label ?? label)
        : t("selected", { count: selected.length })

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="secondary"
          className="w-full justify-between sm:w-auto"
          aria-label={label}
          data-active={selected.length > 0}
        >
          <span className="truncate">{summary}</span>
          <ChevronDown aria-hidden />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 p-1">
        <Command>
          <CommandInput placeholder={t("findOption")} />
          <CommandList>
            <CommandEmpty>{t("noOptions")}</CommandEmpty>
            <CommandGroup>
              {options.map((o) => (
                <CommandItem key={o.value} value={o.label} onSelect={() => toggle(o.value)}>
                  <Checkbox checked={selected.includes(o.value)} tabIndex={-1} aria-hidden className="pointer-events-none" />
                  {o.label}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

interface MasterSelectProps {
  label: string
  allLabel: string
  value: string | undefined
  options: MasterItem[]
  onChange: (id: string | undefined) => void
}

function MasterSelect({ label, allLabel, value, options, onChange }: MasterSelectProps) {
  return (
    <Select value={value ?? ALL} onValueChange={(v) => onChange(v === ALL ? undefined : v)}>
      <SelectTrigger aria-label={label} className="w-full sm:w-52">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.id} value={o.id}>
            {o.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

interface Props {
  filters: AssetFilters
  onChange: (patch: Partial<AssetFilters>) => void
  onReset: () => void
  /** True when anything beyond sort/page is set; enables Reset. */
  dirty: boolean
}

/** Search box and the four filter controls. Search is debounced; the rest apply immediately. */
export function AssetFilterBar({ filters, onChange, onReset, dirty }: Props) {
  const t = useTranslations("assets.filters")
  const tCat = useTranslations("category")
  const tStatus = useTranslations("status")

  // Local text so typing is instant; the URL (and the query) follows after the pause.
  const [text, setText] = useState(filters.q)
  const [seenQ, setSeenQ] = useState(filters.q)
  if (filters.q !== seenQ) {
    // The URL changed from outside (Reset, back button): adopt it unless it is just our own trimmed echo.
    setSeenQ(filters.q)
    if (filters.q !== text.trim()) setText(filters.q)
  }
  useEffect(() => {
    const next = text.trim()
    if (next === filters.q) return
    const id = setTimeout(() => onChange({ q: next }), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(id)
    // onChange identity changes with every URL update; only the typed text should re-arm the timer.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text])

  const locations = useQuery({ queryKey: ["locations", ""], queryFn: () => api<MasterItem[]>("/api/locations") })
  const organizations = useQuery({ queryKey: ["organizations", ""], queryFn: () => api<MasterItem[]>("/api/organizations") })

  return (
    <div role="group" aria-label={t("group")} className="flex flex-col gap-3">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-subtle-foreground" aria-hidden />
        <Input
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t("search")}
          aria-label={t("search")}
          className="pl-12"
        />
      </div>
      <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap sm:items-center">
        <MultiSelect
          label={t("category")}
          options={ASSET_CATEGORIES.map((c) => ({ value: c, label: tCat(c) }))}
          selected={filters.category}
          onChange={(category) => onChange({ category })}
        />
        <MultiSelect
          label={t("status")}
          options={ASSET_STATUSES.map((s) => ({ value: s, label: tStatus(s) }))}
          selected={filters.status}
          onChange={(status) => onChange({ status })}
        />
        <MasterSelect
          label={t("location")}
          allLabel={t("allLocations")}
          value={filters.location_id}
          options={locations.data ?? []}
          onChange={(location_id) => onChange({ location_id })}
        />
        <MasterSelect
          label={t("organization")}
          allLabel={t("allOrganizations")}
          value={filters.organization_id}
          options={organizations.data ?? []}
          onChange={(organization_id) => onChange({ organization_id })}
        />
        <Button variant="ghost" onClick={onReset} disabled={!dirty} className="col-span-2 sm:col-span-1">
          <RotateCcw />
          {t("reset")}
        </Button>
      </div>
    </div>
  )
}
