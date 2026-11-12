// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import { NextIntlClientProvider } from "next-intl"
import en from "@/messages/en.json"
import id from "@/messages/id.json"
import { DASHBOARD_LIST_CAP, type DashboardData } from "@/lib/types"
import { AttentionList } from "./attention-list"
import { UpcomingMaintenance } from "./upcoming-maintenance"

function upcoming(n: number): DashboardData["upcoming_maintenance"] {
  return Array.from({ length: n }, (_, i) => ({
    asset_id: `a${i}`,
    asset_tag: `AST-${String(i).padStart(8, "0")}`,
    name: `Asset ${i}`,
    next_date: "2099-01-01",
    overdue: false,
  }))
}

function attention(n: number): DashboardData["attention"] {
  return Array.from({ length: n }, (_, i) => ({
    id: `a${i}`,
    asset_tag: `AST-${String(i).padStart(8, "0")}`,
    name: `Asset ${i}`,
    status: "damaged" as const,
    location_name: null,
    updated_at: "2026-11-12T10:00:00Z",
  }))
}

function wrap(ui: React.ReactNode, locale: "en" | "id" = "en") {
  return render(
    <NextIntlClientProvider locale={locale} messages={locale === "en" ? en : id} timeZone="Asia/Jakarta">
      {ui}
    </NextIntlClientProvider>,
  )
}
afterEach(cleanup)

describe("UpcomingMaintenance remainder", () => {
  it("shows the exact remainder below the API cap", () => {
    wrap(<UpcomingMaintenance items={upcoming(10)} />)
    expect(screen.getByText("and 2 more")).toBeTruthy()
  })

  it("shows n+ when the API returned a full page", () => {
    wrap(<UpcomingMaintenance items={upcoming(DASHBOARD_LIST_CAP)} />)
    expect(screen.getByText("and 42+ more")).toBeTruthy()
  })

  it("uses the same n+ form in Indonesian", () => {
    wrap(<UpcomingMaintenance items={upcoming(DASHBOARD_LIST_CAP)} />, "id")
    expect(screen.getByText("dan 42+ lainnya")).toBeTruthy()
  })
})

describe("AttentionList view all", () => {
  it("counts from the uncapped total, not the returned rows", () => {
    wrap(<AttentionList items={attention(DASHBOARD_LIST_CAP)} total={1200} />)
    expect(screen.getByRole("link", { name: "View all (1,200)" })).toBeTruthy()
  })

  it("hides the link when every row fits", () => {
    wrap(<AttentionList items={attention(3)} total={3} />)
    expect(screen.queryByRole("link", { name: /View all/ })).toBeNull()
  })
})
