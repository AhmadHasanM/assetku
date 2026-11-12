import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"
import { Slot } from "radix-ui"

// StarChart buttons: always pill-shaped. The ::after layer extends the hit
// area to at least 44px tall without changing the visual size.
const buttonVariants = cva(
  "group/button relative inline-flex shrink-0 items-center justify-center rounded-full border-transparent bg-clip-padding font-semibold whitespace-nowrap transition-all outline-none select-none after:absolute after:inset-x-0 after:top-1/2 after:min-h-11 after:h-full after:-translate-y-1/2 after:content-[''] focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-40 disabled:shadow-none aria-invalid:ring-3 aria-invalid:ring-destructive/30 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-[1.125em]",
  {
    variants: {
      variant: {
        // Primary (Star): main CTA only (add, save, submit).
        default:
          "border-0 bg-primary text-primary-foreground hover:brightness-110 hover:glow-star-sm active:brightness-95",
        // Secondary (Nebula): transparent with a 2px nebula outline.
        secondary:
          "border-2 border-nebula-text bg-transparent text-nebula-text hover:bg-nebula/15 hover:glow-nebula-sm aria-expanded:bg-nebula/15",
        ghost:
          "border-0 bg-transparent text-muted-foreground hover:bg-nebula/10 hover:text-foreground aria-expanded:bg-nebula/10 aria-expanded:text-foreground",
        destructive:
          "border-0 bg-destructive text-destructive-foreground hover:brightness-110 hover:glow-destructive active:brightness-95 focus-visible:ring-destructive/40",
      },
      size: {
        sm: "h-8 gap-1.5 px-4 py-1.5 text-sm",
        default: "h-10 gap-2 px-6 py-2.5 text-base",
        lg: "h-12 gap-2 px-8 py-3.5 text-lg",
        icon: "size-10 after:-inset-x-0.5",
        "icon-sm": "size-8 after:-inset-x-1.5",
        "icon-lg": "size-12",
        // Compact sizes used inside composite shadcn parts (input groups).
        xs: "h-6 gap-1 px-2 text-xs",
        "icon-xs": "size-6",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
