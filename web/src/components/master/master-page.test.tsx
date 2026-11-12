// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"

const { apiMock, toast } = vi.hoisted(() => ({
  apiMock: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), api: apiMock }))
vi.mock("sonner", () => ({ toast }))

import { ApiError } from "@/lib/api"
import { MasterPage } from "./master-page"

const ROWS = [
  { id: "1", name: "Head Office", description: "HQ building", asset_count: 2 },
  { id: "2", name: "Warehouse", description: "", asset_count: 0 },
]

function renderPage(kind: "locations" | "suppliers" = "locations") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <QueryClientProvider client={qc}>
        <MasterPage kind={kind} />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  )
}

beforeAll(() => {
  // jsdom lacks these; Radix dialogs and pointer interactions need them.
  Element.prototype.hasPointerCapture ??= () => false
  Element.prototype.scrollIntoView ??= () => {}
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
})

beforeEach(() => {
  apiMock.mockReset()
  toast.success.mockReset()
  toast.error.mockReset()
  apiMock.mockImplementation(async (path: string, init?: { method?: string }) => {
    if (!init?.method && path.startsWith("/api/locations")) return ROWS
    return undefined
  })
})
afterEach(cleanup)

describe("MasterPage", () => {
  it("renders the list from the API", async () => {
    renderPage()
    expect((await screen.findAllByText("Head Office")).length).toBeGreaterThan(0)
    expect(screen.getAllByText("Warehouse").length).toBeGreaterThan(0)
    expect(apiMock).toHaveBeenCalledWith("/api/locations")
  })

  it("submits the add form with POST", async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findAllByText("Head Office")
    await user.click(screen.getByRole("button", { name: "Add" }))
    await user.type(await screen.findByLabelText("Name"), "Branch B")
    await user.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() =>
      expect(apiMock).toHaveBeenCalledWith("/api/locations", {
        method: "POST",
        json: { name: "Branch B", description: "" },
      }),
    )
    await waitFor(() => expect(toast.success).toHaveBeenCalled())
  })

  it("shows a taken name error under the field", async () => {
    apiMock.mockImplementation(async (_p: string, init?: { method?: string }) => {
      if (init?.method === "POST") throw new ApiError(422, "validation", "validation failed", { name: "taken" })
      return ROWS
    })
    const user = userEvent.setup()
    renderPage()
    await screen.findAllByText("Head Office")
    await user.click(screen.getByRole("button", { name: "Add" }))
    await user.type(await screen.findByLabelText("Name"), "Warehouse")
    await user.click(screen.getByRole("button", { name: "Save" }))
    const dialog = await screen.findByRole("dialog")
    expect(await within(dialog).findByText("Already in use.")).toBeTruthy()
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("shows the asset count in a toast when delete returns 409 in_use", async () => {
    apiMock.mockImplementation(async (_p: string, init?: { method?: string }) => {
      if (init?.method === "DELETE") throw new ApiError(409, "in_use", "still referenced", { count: "2" })
      return ROWS
    })
    const user = userEvent.setup()
    renderPage()
    await screen.findAllByText("Head Office")
    await user.click(screen.getAllByRole("button", { name: "Delete Head Office" })[0])
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Still used by 2 assets."))
  })

  it("uses a generic message when in_use has count 0", async () => {
    apiMock.mockImplementation(async (_p: string, init?: { method?: string }) => {
      if (init?.method === "DELETE") throw new ApiError(409, "in_use", "still referenced", { count: "0" })
      return ROWS
    })
    const user = userEvent.setup()
    renderPage()
    await screen.findAllByText("Warehouse")
    await user.click(screen.getAllByRole("button", { name: "Delete Warehouse" })[0])
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Still referenced by other records."))
  })

  it("sends the search term to the API", async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findAllByText("Head Office")
    await user.type(screen.getByRole("searchbox"), "ware")
    await waitFor(() => expect(apiMock).toHaveBeenCalledWith("/api/locations?q=ware"))
  })

  it("renders the supplier fields", async () => {
    apiMock.mockResolvedValue([])
    const user = userEvent.setup()
    renderPage("suppliers")
    await screen.findByText("No data yet")
    await user.click(screen.getByRole("button", { name: "Add" }))
    for (const label of ["Name", "Contact name", "Phone", "Email", "Address"]) {
      expect(await screen.findByLabelText(new RegExp(`^${label}`))).toBeTruthy()
    }
  })
})
