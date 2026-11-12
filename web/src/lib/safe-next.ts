const FALLBACK = "/dashboard"
const ADMIN_FALLBACK = "/admin/tenants"

/**
 * Returns next only when it is a same-origin relative path ("/..." but not
 * "//..."), with no backslashes (raw or %5C) and no control characters
 * (browsers strip tabs/newlines, turning "/\t/evil.com" into "//evil.com").
 */
export function safeNext(next: string | null | undefined, fallback: string = FALLBACK): string {
  if (!next || !next.startsWith("/") || next.startsWith("//")) return fallback
  if (next.includes("\\") || /%5c/i.test(next)) return fallback
  if (/[\u0000-\u001f\u007f]/.test(next)) return fallback
  return next
}

/** Like safeNext, but only admin panel paths (never the admin login itself) are kept. */
export function safeAdminNext(next: string | null | undefined): string {
  const path = safeNext(next, ADMIN_FALLBACK)
  if (!/^\/admin(?:[/?#]|$)/.test(path) || /^\/admin\/login(?:[/?#]|$)/.test(path)) return ADMIN_FALLBACK
  return path
}
