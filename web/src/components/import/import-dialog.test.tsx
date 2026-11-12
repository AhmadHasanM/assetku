// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { act, cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import { ApiError } from "@/lib/api"
import type { ImportJob } from "@/lib/types"

const { apiMock, apiBlobMock, toast } = vi.hoisted(() => ({
  apiMock: vi.fn(),
  apiBlobMock: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))
vi.mock("@/lib/api", async (orig) => ({
  ...(await orig<typeof import("@/lib/api")>()),
  api: apiMock,
  apiBlob: apiBlobMock,
}))
vi.mock("sonner", () => ({ toast }))

import { ImportDialog } from "./import-dialog"
import { closeImportDialog, openImportDialog, resetImportStore } from "./import-store"

const RESULT = {
  created: 3,
  updated: 2,
  unchanged: 1,
  skipped: 1,
  unknown_columns: ["Ruangan"],
  unmatched_organizations: [{ name: "PT Lain", rows: 2 }],
  unrecognized_values: [{ field: "status", value: "Bagus", rows: 1 }],
  skipped_rows: [{ row: 7, reason: "missing_name" }],
  created_suppliers: ["CV Baru"],
  created_locations: ["Gudang Bekasi"],
}

function job(patch: Partial<ImportJob>): ImportJob {
  return { id: "j1", status: "running", total_rows: 10, processed_rows: 0, result: null, error: "", ...patch }
}

function renderDialog() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <QueryClientProvider client={qc}>
        <ImportDialog />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  )
}

function csvFile(size?: number) {
  const f = new File(["Nama Barang;Harga\nLaptop;Rp 12.500.000\n"], "assets.csv", { type: "text/csv" })
  if (size !== undefined) Object.defineProperty(f, "size", { value: size })
  return f
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
  vi.useFakeTimers({ shouldAdvanceTime: true })
  apiMock.mockReset()
  apiBlobMock.mockReset()
  Object.values(toast).forEach((m) => m.mockReset())
  resetImportStore()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

function setup() {
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
  renderDialog()
  act(() => openImportDialog())
  return user
}

async function pickAndStart(user: ReturnType<typeof userEvent.setup>, file = csvFile()) {
  await user.upload(screen.getByLabelText("Choose file"), file)
  await user.click(screen.getByRole("button", { name: "Start Import" }))
}

describe("ImportDialog", () => {
  it("uploads the file, polls every second and shows progress then the result counts", async () => {
    const polls: ImportJob[] = [
      job({ processed_rows: 5 }),
      job({ status: "done", processed_rows: 10, result: RESULT }),
    ]
    apiMock.mockImplementation(async (path: string, init?: { method?: string; body?: FormData }) => {
      if (path === "/api/imports" && init?.method === "POST") return { id: "j1" }
      if (path === "/api/imports/j1") return polls.length > 1 ? polls.shift() : polls[0]
      throw new Error(`unexpected ${path}`)
    })
    const user = setup()
    expect(screen.getByRole("link", { name: "Download template" }).getAttribute("href")).toBe(
      "/api/imports/template.xlsx",
    )
    await pickAndStart(user)

    const [, init] = apiMock.mock.calls.find((c) => c[0] === "/api/imports")!
    expect((init.body as FormData).get("file")).toBeInstanceOf(File)
    expect(sessionStorage.getItem("assetku.import.job")).toBe("j1")

    expect(await screen.findByText("50%")).toBeTruthy()
    expect(screen.getByText("5 of 10 rows")).toBeTruthy()
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("50")

    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(await screen.findByText("Import finished")).toBeTruthy()
    const pollCount = apiMock.mock.calls.filter((c) => c[0] === "/api/imports/j1").length
    expect(pollCount).toBe(2)
    for (const [label, n] of [
      ["New", "3"],
      ["Updated", "2"],
      ["Unchanged", "1"],
      ["Skipped", "1"],
    ]) {
      expect(screen.getByTestId(`count-${label}`).textContent).toContain(n)
    }
    // A finished job is no longer resumed after reload, and no toast while the dialog is open.
    expect(sessionStorage.getItem("assetku.import.job")).toBeNull()
    expect(toast.success).not.toHaveBeenCalled()

    // Polling stops once the job is final.
    await act(() => vi.advanceTimersByTimeAsync(3000))
    expect(apiMock.mock.calls.filter((c) => c[0] === "/api/imports/j1").length).toBe(2)

    // Collapsible details show translated skip reasons.
    await user.click(screen.getByRole("button", { name: /Skipped rows/ }))
    expect(screen.getByText("Row 7")).toBeTruthy()
    expect(screen.getByText("Asset name is empty")).toBeTruthy()
  })

  it("keeps polling after the dialog closes and toasts when done", async () => {
    let n = 0
    apiMock.mockImplementation(async (path: string, init?: { method?: string }) => {
      if (path === "/api/imports" && init?.method === "POST") return { id: "j1" }
      if (path === "/api/imports/j1") {
        n++
        return n < 3 ? job({ processed_rows: n }) : job({ status: "done", processed_rows: 10, result: RESULT })
      }
      throw new Error(`unexpected ${path}`)
    })
    const user = setup()
    await pickAndStart(user)
    await screen.findByText("1 of 10 rows")
    act(() => closeImportDialog())
    await waitFor(() => expect(screen.queryByText("1 of 10 rows")).toBeNull())

    await act(() => vi.advanceTimersByTimeAsync(2000))
    await waitFor(() => expect(toast.success).toHaveBeenCalled())
    expect(toast.success.mock.calls[0][0]).toBe("Import finished: 3 new, 2 updated, 1 skipped.")
    expect(sessionStorage.getItem("assetku.import.job")).toBeNull()
  })

  it("resumes a job stored in sessionStorage", async () => {
    sessionStorage.setItem("assetku.import.job", "j9")
    resetImportStore({ keepStorage: true })
    apiMock.mockImplementation(async (path: string) => {
      if (path === "/api/imports/j9") return job({ id: "j9", processed_rows: 3, total_rows: 4 })
      throw new Error(`unexpected ${path}`)
    })
    setup()
    expect(await screen.findByText("75%")).toBeTruthy()
  })

  it("shows the translated header_not_found error on step 1", async () => {
    apiMock.mockRejectedValue(new ApiError(422, "header_not_found", "no recognized header"))
    const user = setup()
    await pickAndStart(user)
    expect(await screen.findByText("Could not find the column header row.")).toBeTruthy()
    expect(screen.getByRole("button", { name: "Start Import" })).toBeTruthy()
    expect(sessionStorage.getItem("assetku.import.job")).toBeNull()
  })

  it("rejects an 11 MB file without calling the server", async () => {
    const user = setup()
    await user.upload(screen.getByLabelText("Choose file"), csvFile(11 * 1024 * 1024))
    expect(await screen.findByText("The file is larger than 10 MB.")).toBeTruthy()
    expect((screen.getByRole("button", { name: "Start Import" }) as HTMLButtonElement).disabled).toBe(true)
    expect(apiMock).not.toHaveBeenCalled()
  })
})
