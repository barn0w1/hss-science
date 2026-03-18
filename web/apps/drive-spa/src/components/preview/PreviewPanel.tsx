import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useUIStore } from "@/store/ui.store"
import { useDriveStore } from "@/store/drive.store"
import { NODES } from "@/mocks/fixtures"
import type { NodeId } from "@/types/domain"
import { GenericPreview } from "./GenericPreview"

export function PreviewPanel() {
  const previewNodeId = useUIStore((s) => s.previewNodeId)
  const closePreview = useUIStore((s) => s.closePreview)
  const mutations = useDriveStore((s) => s.mutations)

  const allNodes = [...NODES, ...mutations.created]
  const node = previewNodeId
    ? allNodes.find((n) => n.id === (previewNodeId as NodeId))
    : undefined

  const displayName = node ? (mutations.renamed.get(node.id) ?? node.name) : ""

  return (
    <Sheet
      open={!!previewNodeId}
      onOpenChange={(open) => !open && closePreview()}
    >
      <SheetContent side="right" className="w-80 sm:w-80">
        {node && (
          <>
            <SheetHeader>
              <SheetTitle className="truncate pr-6 text-sm">
                {displayName}
              </SheetTitle>
            </SheetHeader>
            <GenericPreview node={node} />
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}
