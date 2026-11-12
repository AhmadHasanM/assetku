"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { AssetForm } from "@/components/assets/asset-form"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { api, ApiError } from "@/lib/api"
import type { Asset } from "@/lib/types"

export default function EditAssetPage() {
  const t = useTranslations("assetForm")
  const tc = useTranslations("common")
  const { id } = useParams<{ id: string }>()
  const asset = useQuery({
    queryKey: ["assets", "detail", id],
    queryFn: () => api<Asset>(`/api/assets/${encodeURIComponent(id)}`),
    // Edit always starts from the stored values, not a cached copy.
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  })

  // Wait for the fresh fetch, then keep the form mounted through later refetches so edits survive.
  if (asset.data && asset.isFetchedAfterMount) return <AssetForm key={id} asset={asset.data} />

  if (asset.isError) {
    const missing = asset.error instanceof ApiError && asset.error.status === 404
    return (
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <h1 className="text-[28px] sm:text-4xl">{t("editTitle")}</h1>
        <Card className="items-center gap-4 px-6 text-center">
          <p>{missing ? t("notFound") : t("loadFailed")}</p>
          {missing ? (
            <Button asChild variant="secondary">
              <Link href="/assets">{t("back")}</Link>
            </Button>
          ) : (
            <Button variant="secondary" onClick={() => asset.refetch()}>
              {tc("retry")}
            </Button>
          )}
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6" aria-busy="true">
      <Skeleton className="h-10 w-48 rounded-lg" />
      <Skeleton className="h-96 rounded-lg" />
      <Skeleton className="h-64 rounded-lg" />
    </div>
  )
}
