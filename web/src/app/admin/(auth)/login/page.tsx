import type { Metadata } from "next"
import { getTranslations } from "next-intl/server"
import { LoginForm } from "@/components/auth/login-form"
import { Logo } from "@/components/shell/logo"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { PRODUCT_NAME } from "@/lib/brand"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin")
  return { title: t("loginTitle") }
}

export default async function AdminLoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>
}) {
  const t = await getTranslations("admin")
  const { next } = await searchParams
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-8 px-4 py-10">
      <Logo label={`${PRODUCT_NAME} Admin`} className="text-[28px]" />
      <Card className="w-full max-w-md glow-nebula-md">
        <CardHeader>
          <CardTitle>
            <h1 className="text-[28px] leading-tight">{t("loginTitle")}</h1>
          </CardTitle>
          <CardDescription>{t("loginSubtitle")}</CardDescription>
        </CardHeader>
        <CardContent>
          <LoginForm admin next={typeof next === "string" ? next : null} />
        </CardContent>
      </Card>
    </main>
  )
}
