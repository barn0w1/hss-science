import { useEffect, useState } from "react"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"
import type { DriveNode } from "@/types/domain"
import { NodeGrid } from "./NodeGrid"
import { NodeList } from "./NodeList"
import { ToolBar } from "./ToolBar"
import { SelectionBar } from "./SelectionBar"
import { PreviewPanel } from "@/components/preview/PreviewPanel"
import { NewFolderDialog } from "@/components/dialogs/NewFolderDialog"
import { DeleteConfirmDialog } from "@/components/dialogs/DeleteConfirmDialog"
import { ShareDialog } from "@/components/dialogs/ShareDialog"
import { Skeleton } from "@/components/ui/skeleton"
import { FolderOpen } from "lucide-react"

function sortNodes(nodes: DriveNode[], field: string, dir: string): DriveNode[] {
  return [...nodes].sort((a, b) => {
    if (a.kind === "folder" && b.kind !== "folder") return -1
    if (a.kind !== "folder" && b.kind === "folder") return 1

    let cmp = 0
    if (field === "name") {
      cmp = a.name.localeCompare(b.name)
    } else if (field === "updatedAt") {
      cmp = a.updatedAt.localeCompare(b.updatedAt)
    } else if (field === "size") {
      const aSize = a.kind === "file" ? a.size : 0
      const bSize = b.kind === "file" ? b.size : 0
      cmp = aSize - bSize
    } else if (field === "kind") {
      cmp = a.kind.localeCompare(b.kind)
    }
    return dir === "asc" ? cmp : -cmp
  })
}

export function DriveView() {
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const currentNodeId = useDriveStore((s) => s.currentNodeId)
  const clearSelection = useDriveStore((s) => s.clearSelection)
  const mutations = useDriveStore((s) => s.mutations)

  const viewMode = useUIStore((s) => s.viewMode)
  const sortField = useUIStore((s) => s.sortField)
  const sortDir = useUIStore((s) => s.sortDir)
  const searchQuery = useUIStore((s) => s.searchQuery)

  const [nodes, setNodes] = useState<DriveNode[]>([])
  const [isLoading, setIsLoading] = useState(false)

  useEffect(() => {
    if (!currentSpaceId) return
    const params = new URLSearchParams()
    if (currentNodeId) params.set("parentId", currentNodeId)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsLoading(true)
    let cancelled = false
    fetch(`/api/spaces/${currentSpaceId}/nodes?${params}`)
      .then((r) => r.json())
      .then((data: { items: DriveNode[] }) => {
        if (!cancelled) {
          setNodes(data.items)
          setIsLoading(false)
        }
      })
      .catch(() => {
        if (!cancelled) setIsLoading(false)
      })
    return () => { cancelled = true }
  }, [currentSpaceId, currentNodeId])

  const visibleNodes = sortNodes(
    [...nodes, ...mutations.created.filter(
      (n) => n.spaceId === currentSpaceId && n.parentId === currentNodeId,
    )].filter(
      (n) =>
        !mutations.deleted.has(n.id) &&
        n.name.toLowerCase().includes(searchQuery.toLowerCase()),
    ),
    sortField,
    sortDir,
  )

  return (
    <div className="flex flex-1 overflow-hidden" onClick={() => clearSelection()}>
      <div className="flex flex-1 flex-col overflow-hidden">
        <ToolBar />
        <div className="flex flex-1 flex-col overflow-auto p-2">
          {isLoading ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-1 p-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="h-[120px] rounded-xl" />
              ))}
            </div>
          ) : visibleNodes.length === 0 ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 py-16 text-center">
              <FolderOpen className="h-16 w-16 text-muted-foreground/20" />
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  {searchQuery ? "No files match your search" : "This folder is empty"}
                </p>
                {!searchQuery && (
                  <p className="mt-1 text-xs text-muted-foreground/70">
                    Create a folder or drop files to get started
                  </p>
                )}
              </div>
            </div>
          ) : viewMode === "grid" ? (
            <NodeGrid nodes={visibleNodes} />
          ) : (
            <NodeList nodes={visibleNodes} />
          )}
        </div>
        <div className="border-t border-border px-4 py-2">
          <SelectionBar />
        </div>
      </div>
      <PreviewPanel />
      <NewFolderDialog />
      <DeleteConfirmDialog />
      <ShareDialog />
    </div>
  )
}
