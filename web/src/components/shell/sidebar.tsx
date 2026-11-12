import Link from "next/link"
import { Logo } from "./logo"
import { NavLinks } from "./nav-links"

/** Desktop sidebar (>= 1024px). Below that, MobileNav's Sheet replaces it. */
export function Sidebar() {
  return (
    <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col gap-5 border-r border-sidebar-border bg-sidebar px-3 py-5 lg:flex">
      <Link href="/dashboard" className="flex min-h-11 items-center rounded-md px-3 outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <Logo />
      </Link>
      <NavLinks />
    </aside>
  )
}
