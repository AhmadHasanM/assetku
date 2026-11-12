// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest"
import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { NextIntlClientProvider } from "next-intl"
import messages from "@/messages/en.json"
import { SummaryCards } from "./summary-cards"

const SUMMARY = { total: 45, available: 20, in_use: 15, needs_attention: 8 }

function setup(active: Parameters<typeof SummaryCards>[0]["active"] = "total") {
  const onChange = vi.fn()
  render(
    <NextIntlClientProvider locale="en" messages={messages}>
      <SummaryCards summary={SUMMARY} active={active} onChange={onChange} />
    </NextIntlClientProvider>,
  )
  return onChange
}
afterEach(cleanup)

describe("SummaryCards", () => {
  it("shows the counts", () => {
    setup()
    expect(screen.getByRole("button", { name: /Needs Attention/ }).textContent).toContain("8")
    expect(screen.getByRole("button", { name: /Total Assets/ }).textContent).toContain("45")
  })

  it("formats counts with the locale's thousands separator", () => {
    render(
      <NextIntlClientProvider locale="en" messages={messages}>
        <SummaryCards summary={{ ...SUMMARY, total: 8055 }} active={null} onChange={vi.fn()} />
      </NextIntlClientProvider>,
    )
    expect(screen.getByRole("button", { name: /Total Assets/ }).textContent).toContain("8,055")
  })

  it("clicking Needs Attention selects the three attention statuses", async () => {
    const onChange = setup()
    await userEvent.setup().click(screen.getByRole("button", { name: /Needs Attention/ }))
    expect(onChange).toHaveBeenCalledWith(["maintenance", "damaged", "lost"])
  })

  it("clicking Total clears the status filter", async () => {
    const onChange = setup("available")
    await userEvent.setup().click(screen.getByRole("button", { name: /Total Assets/ }))
    expect(onChange).toHaveBeenCalledWith([])
  })

  it("marks only the active card as pressed", () => {
    setup("attention")
    expect(screen.getByRole("button", { name: /Needs Attention/ }).getAttribute("aria-pressed")).toBe("true")
    expect(screen.getByRole("button", { name: /Total Assets/ }).getAttribute("aria-pressed")).toBe("false")
  })

  it("marks no card when the status filter matches none", () => {
    setup(null)
    expect(screen.getAllByRole("button").every((b) => b.getAttribute("aria-pressed") === "false")).toBe(true)
  })
})
