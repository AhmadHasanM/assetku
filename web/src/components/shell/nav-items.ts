import { Building2, LayoutDashboard, MapPin, Package, Truck, type LucideIcon } from "lucide-react"

export interface NavItem {
  href: string
  /** Key under the "nav" messages namespace. */
  labelKey: "dashboard" | "assets" | "locations" | "organizations" | "suppliers"
  icon: LucideIcon
}

export const NAV_ITEMS: readonly NavItem[] = [
  { href: "/dashboard", labelKey: "dashboard", icon: LayoutDashboard },
  { href: "/assets", labelKey: "assets", icon: Package },
  { href: "/locations", labelKey: "locations", icon: MapPin },
  { href: "/organizations", labelKey: "organizations", icon: Building2 },
  { href: "/suppliers", labelKey: "suppliers", icon: Truck },
]

export function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`)
}
