"use client"

import { useState } from "react"
import Link from "next/link"
import { useParams, useRouter } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { useTranslations } from "next-intl"
import { ArrowLeft, SearchX } from "lucide-react"
import { AssetHeader } from "@/components/assets/detail/asset-header"
import { BastDialog } from "@/components/assets/detail/bast-dialog"
import { HistoryTab } from "@/components/assets/detail/history-tab"
import { InfoTab } from "@/components/assets/detail/info-tab"
import { MaintenanceTab } from "@/components/assets/detail/maintenance-tab"
import { MovementsTab } from "@/components/assets/detail/movements-tab"
import { DeleteAssetDialog } from "@/components/assets/delete-asset-dialog"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { api, ApiError } from "@/lib/api"
import type { Asset } from "@/lib/types"

const TABS = ["info", "history", "maintenance", "movements"] as const

export default function AssetDetailPage() {
  const t = useTranslations("assetDetail")
  const tc = useTranslations("common")
  const router = useRouter()
  const { id } = useParams<{ id: string }>()
  const [bastOpen, setBastOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  // Set once the asset is deleted: stop fetching it while the router leaves the page.
  const [deleted, setDeleted] = useState(false)

  const asset = useQuery({
    queryKey: ["assets", "detail", id],
    queryFn: () => api<Asset>(`/api/assets/${encodeURIComponent(id)}`),
    enabled: !deleted,
  })

  if (deleted) return <DetailSkeleton />

  if (asset.isError) {
    // A missing id, a malformed id and another tenant's asset are all 404.
    const missing = asset.error instanceof ApiError && asset.error.status === 404
    return (
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
        <Card className="items-center gap-4 px-6 py-12 text-center">
          {missing && <SearchX className="size-10 text-nebula-text" aria-hidden />}
          <h1 className="text-[28px]">{missing ? t("notFoundTitle") : t("loadFailed")}</h1>
          {missing && <p className="max-w-md text-muted-foreground">{t("notFoundHint")}</p>}
          {missing ? (
            <Button asChild variant="secondary">
              <Link href="/assets">
                <ArrowLeft />
                {t("back")}
              </Link>
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

  if (!asset.data) return <DetailSkeleton />
  const a = asset.data

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <AssetHeader asset={a} onBast={() => setBastOpen(true)} onDelete={() => setConfirmDelete(true)} />

      <Tabs defaultValue="info" className="gap-4">
        {/* Scrolls sideways on narrow screens instead of squeezing the labels. */}
        <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
          <TabsList className="h-auto w-max gap-1 rounded-full p-1">
            {TABS.map((tab) => (
              <TabsTrigger key={tab} value={tab} className="h-11 flex-none rounded-full px-5 text-base">
                {t(`tabs.${tab}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        <TabsContent value="info">
          <InfoTab asset={a} />
        </TabsContent>
        <TabsContent value="history">
          <HistoryTab assetId={a.id} />
        </TabsContent>
        <TabsContent value="maintenance">
          <MaintenanceTab assetId={a.id} />
        </TabsContent>
        <TabsContent value="movements">
          <MovementsTab asset={a} />
        </TabsContent>
      </Tabs>

      <BastDialog asset={a} open={bastOpen} onOpenChange={setBastOpen} />
      <DeleteAssetDialog
        asset={confirmDelete ? a : null}
        onClose={() => setConfirmDelete(false)}
        onDeleted={() => {
          setDeleted(true)
          router.replace("/assets")
        }}
      />
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6" aria-busy="true">
      <Skeleton className="h-56 rounded-lg" />
      <Skeleton className="h-12 w-80 max-w-full rounded-full" />
      <Skeleton className="h-96 rounded-lg" />
    </div>
  )
}
