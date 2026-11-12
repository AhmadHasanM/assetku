import path from "node:path"
import { expect, test, type Page } from "@playwright/test"

// Seeded by `server seed-demo` (resets the user to Indonesian + dark theme on every run).
const EMAIL = process.env.E2E_EMAIL ?? "demo@assetku.test"
const PASSWORD = process.env.E2E_PASSWORD ?? "demo12345"
const MISSING_ID = "00000000-0000-4000-8000-000000000000"
const MESSY_CSV = path.join(__dirname, "fixtures", "messy.csv")
const ATTENTION = ["maintenance", "damaged", "lost"]

async function login(page: Page) {
  await page.getByLabel("Email").fill(EMAIL)
  await page.getByLabel("Password").fill(PASSWORD)
  await page.getByRole("button", { name: /^(Masuk|Log in)$/ }).click()
}

/** The summary card's number ("1.234" in id, "1,234" in en). */
async function cardCount(page: Page, name: RegExp): Promise<number> {
  const card = page.getByRole("group", { name: /Ringkasan asset|Asset summary/ }).getByRole("button", { name })
  await expect(card).toContainText(/\d/)
  return Number((await card.innerText()).replace(/\D/g, ""))
}

function statusParam(page: Page): string[] {
  return (new URL(page.url()).searchParams.get("status") ?? "").split(",").filter(Boolean)
}

/** Radix sets pointer-events:none on <body> while a modal is open; it must be gone after closing. */
async function expectPageInteractive(page: Page) {
  await expect.poll(() => page.evaluate(() => getComputedStyle(document.body).pointerEvents)).not.toBe("none")
}

test("golden path", async ({ page, context }) => {
  await test.step("1. QR scan while logged out returns to the asset after login", async () => {
    await page.goto(`/assets/${MISSING_ID}`)
    await expect(page).toHaveURL(/\/login\?next=/)
    expect(new URL(page.url()).searchParams.get("next")).toBe(`/assets/${MISSING_ID}`)
    await login(page)
    await expect(page).toHaveURL(new RegExp(`/assets/${MISSING_ID}$`))
    await expect(page.getByRole("heading", { name: "Asset tidak ditemukan" })).toBeVisible()
  })

  await test.step("2. an external next is ignored", async () => {
    await context.clearCookies()
    await page.goto("/login?next=//evil.com")
    await login(page)
    await expect(page).toHaveURL(/\/dashboard$/)
  })

  await test.step("3. messy CSV import creates 5 assets", async () => {
    await page.goto("/assets")
    // The header button (an empty list also offers one).
    await page.getByRole("button", { name: "Import", exact: true }).first().click()
    const dialog = page.getByRole("dialog", { name: "Import Asset" })
    await dialog.getByLabel("Pilih file").setInputFiles(MESSY_CSV)
    await dialog.getByRole("button", { name: "Mulai Import" }).click()
    await expect(dialog.getByTestId("count-Baru")).toContainText("5", { timeout: 30_000 })
    await expect(dialog.getByTestId("count-Dilewati")).toContainText("0")
    await dialog.getByRole("button", { name: "Tutup" }).last().click()
    await expect(dialog).toBeHidden()
  })

  await test.step("4. Needs Attention card filters the list", async () => {
    const expected = await cardCount(page, /Perlu Perhatian/)
    expect(expected).toBeGreaterThanOrEqual(2)
    await page.getByRole("button", { name: /Perlu Perhatian/ }).click()
    await expect.poll(() => statusParam(page)).toEqual(ATTENTION)
    await expect(page.locator("table tbody tr")).toHaveCount(Math.min(expected, 20))
  })

  let assetUrl = ""
  await test.step("5. log a movement to Gudang Bekasi", async () => {
    await page.goto(`/assets?q=${encodeURIComponent("Laptop Dell Latitude 5420")}`)
    await page.locator("table tbody tr").first().click()
    await expect(page).toHaveURL(/\/assets\/[0-9a-f-]{36}$/)
    assetUrl = page.url()
    await expect(page.getByRole("heading", { level: 1, name: "Laptop Dell Latitude 5420" })).toBeVisible()

    await page.getByRole("tab", { name: "Perpindahan" }).click()
    await page.getByRole("button", { name: "Log Movement" }).click()
    const dialog = page.getByRole("dialog", { name: "Log Movement" })
    await dialog.getByRole("combobox", { name: "Lokasi tujuan" }).click()
    await page.getByPlaceholder("Cari atau ketik nama baru…").fill("Gudang")
    await page.getByRole("option", { name: "Gudang Bekasi", exact: true }).click()
    await dialog.getByRole("button", { name: "Simpan" }).click()
    await expect(dialog).toBeHidden()

    const location = page.locator("dl > div").filter({ has: page.locator("dt", { hasText: /^Lokasi$/ }) }).locator("dd")
    await expect(location.first()).toHaveText("Gudang Bekasi")
    await page.getByRole("tab", { name: "Riwayat" }).click()
    await expect(page.getByRole("tabpanel")).toContainText(/memindahkan asset: .*→ Gudang Bekasi/)
  })

  await test.step("6. Generate Label downloads label-*.pdf", async () => {
    const download = page.waitForEvent("download")
    await page.getByRole("button", { name: "Generate Label" }).click()
    expect((await download).suggestedFilename()).toMatch(/^label-.+\.pdf$/)
  })

  await test.step("desktop sidebar stays put after scrolling (dashboard is the longest page)", async () => {
    await page.goto("/dashboard")
    // The composition charts (and their screen-reader tables) are rendered once data is in.
    await expect(page.locator("[data-slot=chart]").first()).toBeVisible()
    const sidebar = page.locator("aside").first()
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight))
    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0)
    await expect.poll(async () => (await sidebar.boundingBox())?.y).toBe(0)
  })

  await test.step("7. switch language to English", async () => {
    await page.getByRole("button", { name: "Menu pengguna" }).click()
    await page.getByRole("menuitem", { name: "Bahasa" }).click()
    await page.getByRole("menuitemradio", { name: "English" }).click()
    await page.goto("/assets")
    await expect(page.getByRole("heading", { level: 1, name: "Assets" })).toBeVisible()
    const sidebar = page.locator("aside").first()
    await expect(sidebar.getByRole("link", { name: "Assets" })).toBeVisible()
    await expect(sidebar.getByRole("link", { name: "Locations" })).toBeVisible()
    await expect(sidebar.getByRole("link", { name: "Suppliers" })).toBeVisible()
  })

  await test.step("8. switch theme to light", async () => {
    await expect(page.locator("html")).toHaveClass(/\bdark\b/)
    await page.getByRole("button", { name: "User menu" }).click()
    await page.getByRole("menuitem", { name: "Theme" }).click()
    await page.getByRole("menuitemradio", { name: "Light" }).click()
    await expect(page.locator("html")).not.toHaveClass(/\bdark\b/)
  })

  await test.step("9. Needs Attention at 360x740: cards and Sheet menu", async () => {
    await page.setViewportSize({ width: 360, height: 740 })
    await page.goto("/assets")
    const expected = await cardCount(page, /Needs Attention/)
    await page.getByRole("button", { name: /Needs Attention/ }).click()
    await expect.poll(() => statusParam(page)).toEqual(ATTENTION)
    const cards = page.locator("main ul > li")
    await expect(cards).toHaveCount(Math.min(expected, 20))
    await expect(page.locator("table")).toBeHidden()

    await page.getByRole("button", { name: "Open menu" }).click()
    const sheet = page.getByRole("dialog")
    await expect(sheet.getByRole("link", { name: "Locations" })).toBeVisible()
    await page.keyboard.press("Escape")
    await expect(sheet).toBeHidden()
    await expectPageInteractive(page)
  })

  await test.step("360px: Print BAST from the actions menu leaves the page clickable", async () => {
    await page.goto(assetUrl)
    await page.getByRole("button", { name: "Asset actions" }).click()
    await page.getByRole("menuitem", { name: "Print BAST" }).click()
    const dialog = page.getByRole("dialog", { name: "Print BAST" })
    await expect(dialog).toBeVisible()
    await dialog.getByRole("button", { name: "Cancel" }).click()
    await expect(dialog).toBeHidden()
    await expectPageInteractive(page)
    // A real click on the page still works.
    await page.getByRole("tab", { name: "History" }).click()
    await expect(page.getByRole("tab", { name: "History" })).toHaveAttribute("aria-selected", "true")
  })
})
