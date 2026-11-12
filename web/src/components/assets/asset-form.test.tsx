// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import type { Asset } from "@/lib/types"

const { apiMock, toast, router } = vi.hoisted(() => ({
  apiMock: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
  router: { push: vi.fn(), replace: vi.fn(), back: vi.fn(), refresh: vi.fn(), prefetch: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), api: apiMock }))
vi.mock("sonner", () => ({ toast }))
vi.mock("next/navigation", () => ({ useRouter: () => router }))

import { ApiError } from "@/lib/api"
import { AssetForm } from "./asset-form"

const ASSET: Asset = {
  id: "a1",
  name: "Laptop",
  asset_tag: "AST-OLD",
  category: "it_equipment",
  manufacturer: "",
  model: "",
  serial_number: "",
  status: "in_use",
  location_id: null,
  location: null,
  assigned_to: "",
  organization_id: null,
  organization: null,
  supplier_id: null,
  supplier: null,
  order_number: "",
  purchase_date: null,
  purchase_cost: null,
  warranty_months: null,
  warranty_expires: null,
  warranty_expired: false,
  notes: "",
  created_at: "",
  updated_at: "",
}

function renderForm(asset?: Asset) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <QueryClientProvider client={qc}>
        <AssetForm asset={asset} />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  )
}

type Init = { method?: string; json?: Record<string, unknown> }
const calls = (method: string) =>
  apiMock.mock.calls.filter(([, init]) => (init as Init | undefined)?.method === method) as [string, Init][]

async function fillRequired(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Asset Name"), "Laptop Dell")
  await user.click(screen.getByRole("combobox", { name: "Category" }))
  await user.click(await screen.findByRole("option", { name: "IT Equipment" }))
}

beforeAll(() => {
  Element.prototype.hasPointerCapture ??= () => false
  Element.prototype.releasePointerCapture ??= () => {}
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
  router.push.mockReset()
  apiMock.mockImplementation(async (path: string, init?: Init) => {
    if (!init?.method) return []
    return { ...ASSET, id: "new-id" }
  })
})
afterEach(cleanup)

describe("AssetForm", () => {
  it("shows required errors and does not submit an empty form", async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole("button", { name: "Save" }))
    expect(await screen.findAllByText("Required.")).toHaveLength(2)
    expect(calls("POST")).toHaveLength(0)
  })

  it("does not send asset_tag when the tag is left blank", async () => {
    const user = userEvent.setup()
    renderForm()
    await fillRequired(user)
    await user.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() => expect(calls("POST")).toHaveLength(1))
    const [path, init] = calls("POST")[0]
    expect(path).toBe("/api/assets")
    expect(init.json).toMatchObject({ name: "Laptop Dell", category: "it_equipment", status: "available" })
    expect(init.json).not.toHaveProperty("asset_tag")
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/assets/new-id"))
    expect(toast.success).toHaveBeenCalled()
  })

  it("shows asset_tag=taken from the API under the Asset Tag field", async () => {
    apiMock.mockImplementation(async (_path: string, init?: Init) => {
      if (init?.method === "POST") throw new ApiError(422, "validation", "validation failed", { asset_tag: "taken" })
      return []
    })
    const user = userEvent.setup()
    renderForm()
    await fillRequired(user)
    const tag = screen.getByLabelText("Asset Tag")
    await user.type(tag, "AST-001")
    await user.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() => expect(tag.getAttribute("aria-invalid")).toBe("true"))
    const field = tag.closest("[data-slot=field]") as HTMLElement
    expect(field.textContent).toContain("Already in use.")
    expect(router.push).not.toHaveBeenCalled()
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("edits with PUT and omits the unchanged tag", async () => {
    const user = userEvent.setup()
    renderForm(ASSET)
    expect((screen.getByLabelText("Asset Name") as HTMLInputElement).value).toBe("Laptop")
    await user.clear(screen.getByLabelText("Asset Name"))
    await user.type(screen.getByLabelText("Asset Name"), "Laptop Baru")
    await user.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() => expect(calls("PUT")).toHaveLength(1))
    const [path, init] = calls("PUT")[0]
    expect(path).toBe("/api/assets/a1")
    expect(init.json).toMatchObject({ name: "Laptop Baru", status: "in_use" })
    expect(init.json).not.toHaveProperty("asset_tag")
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/assets/new-id"))
  })

  it("adds a new location from the combobox and selects it", async () => {
    apiMock.mockImplementation(async (path: string, init?: Init) => {
      if (path === "/api/locations" && init?.method === "POST") return { id: "loc-new", name: "Gudang Baru", asset_count: 0 }
      if (path.startsWith("/api/locations")) return [{ id: "loc-1", name: "Head Office", asset_count: 1 }]
      if (!init?.method) return []
      return { ...ASSET, id: "new-id" }
    })
    const user = userEvent.setup()
    renderForm()
    await fillRequired(user)
    await user.click(screen.getByRole("combobox", { name: "Location" }))
    await user.type(await screen.findByPlaceholderText("Search or type a new name…"), "Gudang Baru")
    await user.click(await screen.findByRole("option", { name: '+ Add "Gudang Baru"' }))
    await waitFor(() => expect(calls("POST")[0]).toEqual(["/api/locations", { method: "POST", json: { name: "Gudang Baru" } }]))
    expect((await screen.findByRole("combobox", { name: "Location" })).textContent).toContain("Gudang Baru")

    await user.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() => expect(calls("POST")).toHaveLength(2))
    expect(calls("POST")[1][1].json).toMatchObject({ location_id: "loc-new" })
  })

  it("previews the warranty end date", async () => {
    renderForm({ ...ASSET, purchase_date: "2026-01-31", warranty_months: 1 })
    expect(screen.getByText("28 Feb 2026")).toBeTruthy()
  })

  it("asks before leaving a dirty form with Cancel", async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(router.push).toHaveBeenCalledWith("/assets")
    router.push.mockReset()

    await user.type(screen.getByLabelText("Asset Name"), "x")
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(await screen.findByRole("alertdialog")).toBeTruthy()
    expect(router.push).not.toHaveBeenCalled()
    await user.click(screen.getByRole("button", { name: "Leave" }))
    expect(router.push).toHaveBeenCalledWith("/assets")
  })
})
