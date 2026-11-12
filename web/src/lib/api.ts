import type { ApiErrorBody, FieldErrors } from "./types"

const CSRF_COOKIE = "csrf_token"
const CSRF_HEADER = "X-CSRF-Token"
const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"])
// Endpoints whose 401 means "wrong credentials", not "session expired".
const NO_REDIRECT_PATHS = ["/api/auth/login", "/api/admin/auth/login"]

/** Error thrown for every non-2xx response; code/fields come from the API envelope. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: FieldErrors

  constructor(status: number, code: string, message: string, fields: FieldErrors = {}) {
    super(message || code)
    this.name = "ApiError"
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export interface ApiInit extends Omit<RequestInit, "body"> {
  body?: BodyInit | null
  /** Serialized as the JSON request body. */
  json?: unknown
}

export function readCookie(name: string): string | null {
  if (typeof document === "undefined") return null
  for (const part of document.cookie.split(";")) {
    const [k, ...rest] = part.trim().split("=")
    if (k === name) return decodeURIComponent(rest.join("="))
  }
  return null
}

// A full page load (not router.push) is intentional: api() runs outside React
// and an expired session should drop all client state.
/* eslint-disable @next/next/no-location-assign-relative-destination */
function loginRedirect(path: string) {
  if (typeof window === "undefined") return
  const { pathname, search } = window.location
  const login = path.startsWith("/api/admin/") ? "/admin/login" : "/login"
  if (pathname === login) return
  window.location.assign(`${login}?next=${encodeURIComponent(pathname + search)}`)
}
/* eslint-enable @next/next/no-location-assign-relative-destination */

/**
 * The API sets the csrf_token cookie on any response. A fresh browser has none
 * yet, so before the first unsafe request (usually login) fetch a cheap GET.
 */
async function csrfToken(): Promise<string | null> {
  const existing = readCookie(CSRF_COOKIE)
  if (existing) return existing
  try {
    await fetch("/api/health", { credentials: "same-origin" })
  } catch {
    // The real request below reports the network error.
  }
  return readCookie(CSRF_COOKIE)
}

async function request(path: string, init: ApiInit = {}): Promise<Response> {
  const { json, headers: initHeaders, ...rest } = init
  const method = (rest.method ?? "GET").toUpperCase()
  const headers = new Headers(initHeaders)
  let body = rest.body
  if (json !== undefined) {
    headers.set("Content-Type", "application/json")
    body = JSON.stringify(json)
  }
  if (!SAFE_METHODS.has(method)) {
    const token = await csrfToken()
    if (token) headers.set(CSRF_HEADER, token)
  }
  let res: Response
  try {
    res = await fetch(path, { credentials: "same-origin", ...rest, method, headers, body })
  } catch (e) {
    throw new ApiError(0, "network", e instanceof Error ? e.message : "network error")
  }
  if (res.ok) return res

  let envelope: ApiErrorBody["error"] | undefined
  try {
    envelope = ((await res.json()) as ApiErrorBody).error
  } catch {
    envelope = undefined
  }
  if (res.status === 401 && !NO_REDIRECT_PATHS.includes(path)) loginRedirect(path)
  throw new ApiError(
    res.status,
    envelope?.code ?? "internal",
    envelope?.message ?? res.statusText,
    envelope?.fields ?? {},
  )
}

/** JSON API call. Returns undefined for 204 No Content. */
export async function api<T = unknown>(path: string, init?: ApiInit): Promise<T> {
  const res = await request(path, init)
  if (res.status === 204) return undefined as T
  const text = await res.text()
  return (text ? JSON.parse(text) : undefined) as T
}

/** Binary API call (PDF, XLSX, CSV). */
export async function apiBlob(path: string, init?: ApiInit): Promise<Blob> {
  const res = await request(path, init)
  return res.blob()
}
