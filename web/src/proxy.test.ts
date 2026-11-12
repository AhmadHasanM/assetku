import { describe, expect, it } from "vitest"
import { NextRequest } from "next/server"
import { proxy } from "./proxy"

function run(path: string, cookie?: string) {
  const headers = cookie ? { cookie } : undefined
  return proxy(new NextRequest(new URL(path, "http://localhost:3000"), { headers }))
}

describe("proxy", () => {
  it("redirects app routes without a session to login with next", () => {
    const res = run("/assets/abc?tab=history")
    expect(res.status).toBe(307)
    expect(res.headers.get("location")).toBe(
      "http://localhost:3000/login?next=%2Fassets%2Fabc%3Ftab%3Dhistory",
    )
  })

  it("lets requests with a session through", () => {
    const res = run("/assets/abc", "session=tok")
    expect(res.headers.get("location")).toBeNull()
    expect(res.headers.get("x-middleware-next")).toBe("1")
  })

  it("does not redirect the login page", () => {
    expect(run("/login?next=%2Fassets").headers.get("location")).toBeNull()
  })

  it("sends admin panel routes to the admin login", () => {
    expect(run("/admin/tenants").headers.get("location")).toBe("http://localhost:3000/admin/login")
    expect(run("/admin").headers.get("location")).toBe("http://localhost:3000/admin/login")
    expect(run("/admin/login").headers.get("location")).toBeNull()
  })
})
