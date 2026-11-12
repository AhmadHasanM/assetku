// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import { ApiError } from "@/lib/api"
import type { AdminUser } from "@/lib/admin"
import { UserDialog, type UserValues } from "./user-dialog"

beforeAll(() => {
  // jsdom lacks these; Radix dialogs need them.
  Element.prototype.hasPointerCapture ??= () => false
  Element.prototype.scrollIntoView ??= () => {}
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
})
afterEach(cleanup)

function renderDialog(onSubmit: (v: Partial<UserValues>) => Promise<void>, user: AdminUser | null = null) {
  return render(
    <NextIntlClientProvider locale="en" messages={messages} timeZone="Asia/Jakarta">
      <UserDialog open onOpenChange={() => {}} user={user} onSubmit={onSubmit} />
    </NextIntlClientProvider>,
  )
}

async function fillCreate(password: string) {
  const u = userEvent.setup()
  await u.type(await screen.findByLabelText("Name"), "Budi Santoso")
  await u.type(screen.getByLabelText("Email"), "budi@example.com")
  await u.type(screen.getByLabelText(/^Position/), "Staf IT")
  await u.type(screen.getByLabelText("Initial password"), password)
  await u.click(screen.getByRole("button", { name: "Save" }))
}

describe("UserDialog", () => {
  it("submits the new user fields", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderDialog(onSubmit)
    await fillCreate("rahasia123")
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        name: "Budi Santoso",
        email: "budi@example.com",
        position: "Staf IT",
        password: "rahasia123",
      }),
    )
  })

  it("shows email=taken from the API under the email field", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new ApiError(422, "validation", "validation failed", { email: "taken" }))
    renderDialog(onSubmit)
    await fillCreate("rahasia123")
    const dialog = await screen.findByRole("dialog")
    expect(await within(dialog).findByText("Already in use.")).toBeTruthy()
    expect(screen.getByLabelText("Email").getAttribute("aria-invalid")).toBe("true")
  })

  it("rejects a password shorter than 8 characters without calling the API", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderDialog(onSubmit)
    await fillCreate("pendek7")
    expect(await screen.findByText("At least 8 characters.")).toBeTruthy()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("rejects a password over 72 bytes (bcrypt limit) on the client", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderDialog(onSubmit)
    await fillCreate("é".repeat(37)) // 37 characters, 74 bytes
    expect(await screen.findByText("Too long.")).toBeTruthy()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("edits name and position only, without a password field", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderDialog(onSubmit, {
      id: "u1",
      name: "Budi",
      email: "budi@example.com",
      position: "",
      active: true,
    })
    const u = userEvent.setup()
    const name = await screen.findByLabelText("Name")
    await u.clear(name)
    await u.type(name, "Budi S")
    expect(screen.queryByLabelText("Initial password")).toBeNull()
    expect((screen.getByLabelText("Email") as HTMLInputElement).readOnly).toBe(true)
    await u.click(screen.getByRole("button", { name: "Save" }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ name: "Budi S", position: "" }))
  })
})
