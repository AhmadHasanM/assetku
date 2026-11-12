import { getTranslations } from "next-intl/server"

// Placeholder so post-login navigation lands somewhere; Task 20 builds the dashboard.
export default async function DashboardPage() {
  const t = await getTranslations("dashboard")
  return <h1 className="text-[28px] sm:text-4xl">{t("title")}</h1>
}
