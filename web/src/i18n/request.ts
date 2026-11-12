import { cookies } from "next/headers"
import { getRequestConfig } from "next-intl/server"
import { LOCALE_COOKIE, toLocale } from "./locale"

// No i18n routing: the locale comes from the "locale" cookie (default "id").
export default getRequestConfig(async () => {
  const store = await cookies()
  const locale = toLocale(store.get(LOCALE_COOKIE)?.value)
  return {
    locale,
    timeZone: "Asia/Jakarta",
    messages: (await import(`../messages/${locale}.json`)).default,
  }
})
