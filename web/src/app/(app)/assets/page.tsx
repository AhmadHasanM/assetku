import { Suspense } from "react"
import { AssetListView } from "@/components/assets/asset-list-view"

// useSearchParams in the view needs a Suspense boundary.
export default function AssetsPage() {
  return (
    <Suspense>
      <AssetListView />
    </Suspense>
  )
}
