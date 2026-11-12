import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { api, apiBlob, ApiError } from "./api"

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

describe("api", () => {
  let fetchMock: ReturnType<typeof vi.fn>
  let location: { pathname: string; search: string; assign: ReturnType<typeof vi.fn> }

  beforeEach(() => {
    fetchMock = vi.fn()
    location = { pathname: "/assets/1", search: "", assign: vi.fn() }
    vi.stubGlobal("fetch", fetchMock)
    vi.stubGlobal("document", { cookie: "locale=id; csrf_token=tok%3D123; other=x" })
    vi.stubGlobal("window", { location })
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("sends the CSRF header and JSON body on POST", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { ok: true }))
    const out = await api<{ ok: boolean }>("/api/things", { method: "POST", json: { a: 1 } })
    expect(out).toEqual({ ok: true })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe("/api/things")
    const headers = new Headers(init.headers)
    expect(headers.get("X-CSRF-Token")).toBe("tok=123")
    expect(headers.get("Content-Type")).toBe("application/json")
    expect(init.body).toBe(JSON.stringify({ a: 1 }))
    expect(init.credentials).toBe("same-origin")
  })

  it("primes the CSRF cookie via /api/health before the first unsafe request", async () => {
    const doc = { cookie: "locale=id" }
    vi.stubGlobal("document", doc)
    fetchMock.mockImplementation(async (url: string) => {
      if (url === "/api/health") {
        doc.cookie = "locale=id; csrf_token=fresh"
        return jsonResponse(200, { ok: true })
      }
      return jsonResponse(200, { user: {} })
    })
    await api("/api/auth/login", { method: "POST", json: {} })
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(["/api/health", "/api/auth/login"])
    expect(new Headers(fetchMock.mock.calls[1][1].headers).get("X-CSRF-Token")).toBe("fresh")
  })

  it("does not send the CSRF header on GET", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, {}))
    await api("/api/things")
    const headers = new Headers(fetchMock.mock.calls[0][1].headers)
    expect(headers.has("X-CSRF-Token")).toBe(false)
  })

  it("returns undefined on 204", async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }))
    await expect(api("/api/auth/logout", { method: "POST" })).resolves.toBeUndefined()
  })

  it("redirects to login with next on 401", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(401, { error: { code: "unauthorized", message: "login required" } }),
    )
    await expect(api("/api/assets/1")).rejects.toBeInstanceOf(ApiError)
    expect(location.assign).toHaveBeenCalledWith("/login?next=%2Fassets%2F1")
  })

  it("sends a 401 from an admin endpoint to the admin login with next", async () => {
    location.pathname = "/admin/tenants/t1"
    fetchMock.mockResolvedValue(jsonResponse(401, { error: { code: "unauthenticated", message: "" } }))
    await expect(api("/api/admin/tenants/t1/users")).rejects.toBeInstanceOf(ApiError)
    expect(location.assign).toHaveBeenCalledWith("/admin/login?next=%2Fadmin%2Ftenants%2Ft1")
  })

  it("keeps the query string in next", async () => {
    location.search = "?tab=history"
    fetchMock.mockResolvedValue(jsonResponse(401, { error: { code: "unauthorized", message: "" } }))
    await expect(api("/api/assets/1")).rejects.toBeInstanceOf(ApiError)
    expect(location.assign).toHaveBeenCalledWith("/login?next=%2Fassets%2F1%3Ftab%3Dhistory")
  })

  it("does not redirect for a failed login", async () => {
    location.pathname = "/login"
    fetchMock.mockResolvedValue(
      jsonResponse(401, { error: { code: "invalid_credentials", message: "bad" } }),
    )
    await expect(
      api("/api/auth/login", { method: "POST", json: {} }),
    ).rejects.toMatchObject({ status: 401, code: "invalid_credentials" })
    expect(location.assign).not.toHaveBeenCalled()
  })

  it("throws ApiError with code and fields", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(422, {
        error: { code: "validation", message: "validation failed", fields: { new_password: "too_short" } },
      }),
    )
    const err = await api("/api/me/password", { method: "POST", json: {} }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 422, code: "validation", fields: { new_password: "too_short" } })
  })

  it("maps non-JSON failures to code internal", async () => {
    fetchMock.mockResolvedValue(new Response("bad gateway", { status: 502 }))
    await expect(api("/api/x")).rejects.toMatchObject({ status: 502, code: "internal", fields: {} })
  })

  it("maps network failures to code network", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"))
    await expect(api("/api/x")).rejects.toMatchObject({ status: 0, code: "network" })
  })
})

describe("apiBlob", () => {
  beforeEach(() => {
    vi.stubGlobal("document", { cookie: "csrf_token=abc" })
    vi.stubGlobal("window", { location: { pathname: "/", search: "", assign: vi.fn() } })
  })
  afterEach(() => vi.unstubAllGlobals())

  it("returns the body as a Blob and sends CSRF on POST", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response("%PDF-1.7", { status: 200, headers: { "Content-Type": "application/pdf" } }),
    )
    vi.stubGlobal("fetch", fetchMock)
    const blob = await apiBlob("/api/assets/labels", { method: "POST", json: { ids: [] } })
    expect(blob).toBeInstanceOf(Blob)
    expect(await blob.text()).toBe("%PDF-1.7")
    expect(new Headers(fetchMock.mock.calls[0][1].headers).get("X-CSRF-Token")).toBe("abc")
  })
})
