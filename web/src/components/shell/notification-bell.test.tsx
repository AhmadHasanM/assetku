// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import type { NotificationList } from "@/lib/types"

const { apiMock, router } = vi.hoisted(() => ({
  apiMock: vi.fn(),
  router: { push: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({
  ...(await orig<typeof import("@/lib/api")>()),
  api: apiMock,
}))
vi.mock("next/navigation", () => ({ useRouter: () => router }))

import { NotificationBell } from "./notification-bell"

const ASSET_ID = "0b6f5d1e-8c1a-4a3b-9f00-1234567890ab"

function list(unread: number): NotificationList {
  return {
    unread,
    items: [
      {
        id: 7,
        type: "maintenance_due",
        title: "Maintenance due",
        body: "Maintenance AC Daikin 1 PK (AST-Q81LX2VB) is due on 18 Nov 2026",
        asset_id: ASSET_ID,
        read_at: null,
        created_at: new Date().toISOString(),
      },
    ],
  }
}

function setup(unread: number) {
  apiMock.mockImplementation(async (path: string) => {
    if (path === "/api/notifications") return list(unread)
    return undefined
  })
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <QueryClientProvider client={qc}>
        <NotificationBell />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  )
}

beforeAll(() => {
  // Radix popper measures the trigger.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
})

afterEach(() => {
  cleanup()
  apiMock.mockReset()
  router.push.mockReset()
})

describe("NotificationBell", () => {
  it("shows the unread count on the badge", async () => {
    setup(3)
    expect(await screen.findByText("3")).toBeTruthy()
    expect(screen.getByRole("button", { name: /3 unread/ })).toBeTruthy()
  })

  it("caps the badge at 99+", async () => {
    setup(150)
    expect(await screen.findByText("99+")).toBeTruthy()
  })

  it("clicking a notification marks it read, then opens the asset", async () => {
    setup(3)
    const user = userEvent.setup()
    await user.click(await screen.findByRole("button", { name: /3 unread/ }))
    await user.click(await screen.findByRole("button", { name: /Maintenance due/ }))
    await waitFor(() => expect(router.push).toHaveBeenCalledWith(`/assets/${ASSET_ID}`))
    expect(apiMock).toHaveBeenCalledWith("/api/notifications/7/read", { method: "POST" })
    const readCall = apiMock.mock.invocationCallOrder[apiMock.mock.calls.findIndex((c) => c[0] === "/api/notifications/7/read")]!
    expect(readCall).toBeLessThan(router.push.mock.invocationCallOrder[0]!)
  })

  it("Mark all as read calls read-all", async () => {
    setup(3)
    const user = userEvent.setup()
    await user.click(await screen.findByRole("button", { name: /3 unread/ }))
    await user.click(await screen.findByRole("button", { name: "Mark all as read" }))
    await waitFor(() =>
      expect(apiMock).toHaveBeenCalledWith("/api/notifications/read-all", { method: "POST" }),
    )
  })
})
