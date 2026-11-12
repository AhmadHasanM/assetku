import { NextResponse, type NextRequest } from "next/server"

// Next.js 16 renamed middleware.ts to proxy.ts. This is only a cheap presence
// check of the httpOnly session cookie; the API validates the session itself.
const SESSION_COOKIE = "session"

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl
  if (request.cookies.get(SESSION_COOKIE)?.value) return NextResponse.next()

  if (pathname === "/admin" || pathname.startsWith("/admin/")) {
    if (pathname === "/admin/login") return NextResponse.next()
    return NextResponse.redirect(new URL("/admin/login", request.url))
  }
  if (pathname === "/login") return NextResponse.next()

  const url = new URL("/login", request.url)
  url.searchParams.set("next", pathname + search)
  return NextResponse.redirect(url)
}

export const config = {
  // Everything except the API, Next internals and static files.
  matcher: ["/((?!api/|_next/|favicon\\.ico|icon\\.svg|robots\\.txt|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico)$).*)"],
}
