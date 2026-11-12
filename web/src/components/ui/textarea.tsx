import * as React from "react"
import { cn } from "cn"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "flex field-sizing-content min-h-24 w-full px-4 py-2.5 leading-6 rounded-md border-2 border-border bg-input text-base text-foreground transition-[border-color,box-shadow] outline-none placeholder:text-subtle-foreground hover:border-nebula-text focus-visible:border-nebula-text focus-visible:glow-nebula-sm disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-border aria-invalid:border-error aria-invalid:glow-error-sm",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
