"use client"

import { useEffect, useId, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { cn } from "cn"
import { Input } from "@/components/ui/input"
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover"
import { api } from "@/lib/api"

const SUGGEST_DEBOUNCE_MS = 200

interface Props extends Omit<React.ComponentProps<"input">, "value" | "onChange"> {
  value: string
  onChange: (value: string) => void
}

/**
 * Free-text input with suggestions from names already used in the tenant
 * (GET /api/assets/assignees?q=, debounced). Typing a new name is fine.
 */
export function AssigneeInput({ value, onChange, onBlur, onKeyDown, className, ...props }: Props) {
  const t = useTranslations("assetForm")
  const listId = useId()
  const [focused, setFocused] = useState(false)
  const [dismissed, setDismissed] = useState(false)
  const [active, setActive] = useState(-1)
  const [q, setQ] = useState(value.trim())
  useEffect(() => {
    const timer = setTimeout(() => setQ(value.trim()), SUGGEST_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [value])

  const suggestions = useQuery({
    queryKey: ["assets", "assignees", q],
    queryFn: () => api<string[]>(`/api/assets/assignees?q=${encodeURIComponent(q)}`),
    enabled: focused,
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  })
  // Hide the exact current value: suggesting what is already typed is noise.
  const options = (suggestions.data ?? []).filter((s) => s !== value.trim())
  const open = focused && !dismissed && options.length > 0

  const choose = (name: string) => {
    onChange(name)
    setDismissed(true)
    setActive(-1)
  }

  const keyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    onKeyDown?.(e)
    if (e.key === "Escape" && open) {
      e.preventDefault()
      setDismissed(true)
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      if (!open) {
        setDismissed(false)
        return
      }
      e.preventDefault()
      const step = e.key === "ArrowDown" ? 1 : -1
      setActive((i) => (i + step + options.length) % options.length)
    } else if (e.key === "Enter" && open && active >= 0 && active < options.length) {
      e.preventDefault()
      choose(options[active])
    }
  }

  return (
    <Popover open={open}>
      <PopoverAnchor asChild>
        <Input
          {...props}
          className={className}
          value={value}
          role="combobox"
          aria-autocomplete="list"
          aria-expanded={open}
          aria-controls={open ? listId : undefined}
          aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
          autoComplete="off"
          onChange={(e) => {
            onChange(e.target.value)
            setDismissed(false)
            setActive(-1)
          }}
          onFocus={() => setFocused(true)}
          onBlur={(e) => {
            setFocused(false)
            setActive(-1)
            onBlur?.(e)
          }}
          onKeyDown={keyDown}
        />
      </PopoverAnchor>
      <PopoverContent
        align="start"
        className="w-(--radix-popover-trigger-width) min-w-56 p-1"
        // Keep focus (and typing) in the input.
        onOpenAutoFocus={(e) => e.preventDefault()}
        onCloseAutoFocus={(e) => e.preventDefault()}
      >
        <ul id={listId} role="listbox" aria-label={t("suggestions")} className="flex flex-col">
          {options.map((name, i) => (
            <li
              key={name}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              // mousedown, not click: it fires before the input blurs and closes the list.
              onMouseDown={(e) => {
                e.preventDefault()
                choose(name)
              }}
              className={cn(
                "flex min-h-11 cursor-default items-center rounded-md px-3 py-2 text-sm select-none hover:bg-muted",
                i === active && "bg-muted",
              )}
            >
              <span className="truncate">{name}</span>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}
