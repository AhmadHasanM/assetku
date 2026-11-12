// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import { ApiError } from "@/lib/api"
import { DEFAULT_FILTERS } from "@/lib/filters"

const { apiBlobMock, downloadMock, toast } = vi.hoisted(() => ({
  apiBlobMock: vi.fn(),
  downloadMock: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), apiBlob: apiBlobMock }))
vi.mock("@/components/assets/detail/download", () => ({ downloadBlob: downloadMock }))
vi.mock("sonner", () => ({ toast }))

import { ExportMenu, exportPath } from "./export-menu"

const FILTERS = { ...DEFAULT_FILTERS, q: "laptop", status: ["in_use"], sort: "name" as const, dir: "asc" as const, page: 3 }

function renderMenu() {
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <ExportMenu filters={FILTERS} as="button" />
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
  apiBlobMock.mockReset()
  downloadMock.mockReset()
  Object.values(toast).forEach((m) => m.mockReset())
})
afterEach(cleanup)

describe("ExportMenu", () => {
  it("exports every page of the active filters", () => {
    expect(exportPath("xlsx", FILTERS)).toBe("/api/assets/export?format=xlsx&q=laptop&status=in_use&sort=name&dir=asc")
    expect(exportPath("csv", DEFAULT_FILTERS)).toBe("/api/assets/export?format=csv")
  })

  it("shows a spinner while running and downloads the file", async () => {
    let resolve!: (b: Blob) => void
    apiBlobMock.mockReturnValue(new Promise<Blob>((r) => (resolve = r)))
    const user = userEvent.setup()
    renderMenu()
    await user.click(screen.getByRole("button", { name: "Export" }))
    await user.click(await screen.findByRole("menuitem", { name: "PDF" }))
    const trigger = screen.getByRole("button", { name: "Export" })
    await waitFor(() => expect(trigger.getAttribute("aria-busy")).toBe("true"))
    expect(trigger.querySelector(".animate-spin")).toBeTruthy()
    expect(apiBlobMock.mock.calls[0][0]).toBe("/api/assets/export?format=pdf&q=laptop&status=in_use&sort=name&dir=asc")

    resolve(new Blob(["%PDF"]))
    await waitFor(() => expect(downloadMock).toHaveBeenCalled())
    expect(downloadMock.mock.calls[0][1]).toMatch(/^assets-\d{8}\.pdf$/)
    await waitFor(() => expect(trigger.getAttribute("aria-busy")).toBe("false"))
  })

  it("toasts the translated export_too_large error", async () => {
    apiBlobMock.mockRejectedValue(new ApiError(422, "export_too_large", "too many"))
    const user = userEvent.setup()
    renderMenu()
    await user.click(screen.getByRole("button", { name: "Export" }))
    await user.click(await screen.findByRole("menuitem", { name: "CSV" }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Too much data to export."))
    expect(downloadMock).not.toHaveBeenCalled()
  })
})
