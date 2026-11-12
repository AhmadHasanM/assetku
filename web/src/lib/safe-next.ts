const FALLBACK = "/dashboard"

/**
 * Returns next only when it is a same-origin relative path ("/..." but not
 * "//..."), with no backslashes (raw or %5C) and no control characters
 * (browsers strip tabs/newlines, turning "/\t/evil.com" into "//evil.com").
 */
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//")) return FALLBACK
  if (next.includes("\\") || /%5c/i.test(next)) return FALLBACK
  if (/[\u0000-\u001f\u007f]/.test(next)) return FALLBACK
  return next
}
