// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"

const { apiMock, apiBlobMock, downloadMock, toast } = vi.hoisted(() => ({
  apiMock: vi.fn(),
  apiBlobMock: vi.fn(),
  downloadMock: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({
  ...(await orig<typeof import("@/lib/api")>()),
  api: apiMock,
  apiBlob: apiBlobMock,
}))
vi.mock("./download", () => ({ downloadBlob: downloadMock }))
vi.mock("sonner", () => ({ toast }))

import { BastDialog } from "./bast-dialog"

const DEFAULTS = {
  number: "BAST-AST-8F2K1Q9Z-20261112",
  date: "2026-11-12",
  first_name: "Rina",
  first_position: "Manager",
  second_name: "",
  warranty: "",
}

function renderDialog() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <QueryClientProvider client={qc}>
        <BastDialog asset={{ id: "a1", asset_tag: "AST-8F2K1Q9Z" }} open onOpenChange={() => {}} />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  )
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
  apiBlobMock.mockReset()
  downloadMock.mockReset()
  apiMock.mockImplementation(async (path: string) => {
    if (path === "/api/assets/a1/bast/defaults") return DEFAULTS
    throw new Error(`unexpected ${path}`)
  })
})
afterEach(cleanup)

describe("BastDialog", () => {
  it("prefills from defaults and renumbers when the date changes", async () => {
    const user = userEvent.setup()
    renderDialog()
    expect(await screen.findByDisplayValue("Rina")).toBeTruthy()
    expect(screen.getByDisplayValue("Manager")).toBeTruthy()
    expect(screen.getByText("BAST-AST-8F2K1Q9Z-20261112")).toBeTruthy()

    await user.click(screen.getByLabelText("Handover Date"))
    await user.selectOptions(await screen.findByRole("combobox", { name: /month/i }), "11")
    await user.click(screen.getByRole("button", { name: /December 1st, 2026/ }))

    expect(await screen.findByText("BAST-AST-8F2K1Q9Z-20261201")).toBeTruthy()
  })

  it("requires the second party name before downloading", async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByDisplayValue("Rina")
    await user.click(screen.getByRole("button", { name: "Download BAST" }))
    expect(await screen.findByText("Required.")).toBeTruthy()
    expect(apiBlobMock).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText("Second Party Name"), "Budi")
    apiBlobMock.mockResolvedValue(new Blob(["%PDF"]))
    await user.click(screen.getByRole("button", { name: "Download BAST" }))
    await waitFor(() => expect(downloadMock).toHaveBeenCalled())
    const [path, init] = apiBlobMock.mock.calls[0]
    expect(path).toBe("/api/assets/a1/bast")
    expect(init.json).toMatchObject({ date: "2026-11-12", second_name: "Budi", first_name: "Rina", condition: "good" })
    expect(downloadMock.mock.calls[0][1]).toBe("BAST-AST-8F2K1Q9Z-20261112.pdf")
  })
})
