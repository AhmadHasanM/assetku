import { Star } from "lucide-react"
import { PRODUCT_NAME } from "@/lib/brand"
import { cn } from "@/lib/utils"

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("flex items-center gap-2 font-heading text-xl font-bold text-foreground", className)}>
      <Star
        aria-hidden
        className="size-[22px] fill-current text-star-text dark:drop-shadow-[0_0_8px_rgb(253_224_71/0.6)]"
      />
      {PRODUCT_NAME}
    </span>
  )
}
