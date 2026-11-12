"use client"

import { useTranslations } from "next-intl"
import { PasswordForm } from "@/components/profile/password-form"
import { ProfileForm } from "@/components/profile/profile-form"
import { Skeleton } from "@/components/ui/skeleton"
import { useMe } from "@/lib/me"

export default function ProfilePage() {
  const t = useTranslations("profile")
  const { data: me } = useMe()
  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-6">
      <h1 className="text-[28px] sm:text-4xl">{t("title")}</h1>
      {me ? <ProfileForm me={me} /> : <Skeleton className="h-96 rounded-lg" />}
      <PasswordForm />
    </div>
  )
}
