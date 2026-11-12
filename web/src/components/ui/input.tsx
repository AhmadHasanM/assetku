import * as React from "react"
import { cn } from "cn"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "w-full min-w-0 px-4 py-2.5 leading-6 rounded-md border-2 border-border bg-input text-base text-foreground transition-[border-color,box-shadow] outline-none placeholder:text-subtle-foreground hover:border-nebula-text focus-visible:border-nebula-text focus-visible:glow-nebula-sm disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:border-border aria-invalid:border-error aria-invalid:glow-error-sm file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground disabled:pointer-events-none",
        className
      )}
      {...props}
    />
  )
}

export { Input }
