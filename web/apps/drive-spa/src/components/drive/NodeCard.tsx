import { memo } from "react"
import { cn } from "@/lib/utils"
import type { DriveNode } from "@/types/domain"
import { NodeIcon } from "./NodeIcon"
import { formatBytes } from "@/lib/format"
import { NodeContextMenu } from "./NodeContextMenu"
import { NodeRenameInput } from "./NodeRenameInput"
import { useDriveStore } from "@/store/drive.store"
import { useNavigate, useParams } from "react-router-dom"
import { useUIStore } from "@/store/ui.store"

interface NodeCardProps {
  node: DriveNode
  isSelected: boolean
  orderedIds: string[]
}

export const NodeCard = memo(function NodeCard({
  node,
  isSelected,
  orderedIds,
}: NodeCardProps) {
  const select = useDriveStore((s) => s.select)
  const navigateTo = useDriveStore((s) => s.navigateTo)
  const renamingNodeId = useDriveStore((s) => s.renamingNodeId)
  const openPreview = useUIStore((s) => s.openPreview)
  const params = useParams()
  const navigate = useNavigate()

  const displayName = useDriveStore((s) => {
    const renamed = s.mutations.renamed.get(node.id)
    return renamed ?? node.name
  })

  const isRenaming = renamingNodeId === node.id

  function handleClick(e: React.MouseEvent) {
    e.stopPropagation()
    select(
      node.id,
      { meta: e.metaKey || e.ctrlKey, shift: e.shiftKey },
      orderedIds as Parameters<typeof select>[2]
    )
    if (node.kind === "file" || node.kind === "symlink") {
      openPreview(node.id)
    }
  }

  function handleDoubleClick(e: React.MouseEvent) {
    e.stopPropagation()
    if (node.kind === "folder") {
      navigateTo(node.id)
      const spaceId = params.spaceId
      navigate(`/drive/${spaceId}/${node.id}`)
    }
  }

  return (
    <NodeContextMenu node={node}>
      <div
        role="button"
        tabIndex={0}
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            if (node.kind === "folder")
              handleDoubleClick(e as unknown as React.MouseEvent)
            else handleClick(e as unknown as React.MouseEvent)
          }
        }}
        className={cn(
          "group relative flex cursor-pointer flex-col items-center gap-2 rounded-xl p-3 transition-colors duration-100 outline-none select-none",
          "hover:bg-accent/60 focus-visible:ring-2 focus-visible:ring-ring",
          isSelected && "bg-primary/10 ring-2 ring-primary/40"
        )}
      >
        <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-muted/50">
          <NodeIcon node={node} size="lg" />
        </div>
        <div className="w-full text-center">
          {isRenaming ? (
            <NodeRenameInput node={node} />
          ) : (
            <p className="truncate text-xs leading-tight font-medium">
              {displayName}
            </p>
          )}
          <p className="mt-0.5 text-[10px] text-muted-foreground">
            {node.kind === "file"
              ? formatBytes(node.size)
              : node.kind === "folder"
                ? `${node.childCount} items`
                : "Shortcut"}
          </p>
        </div>
      </div>
    </NodeContextMenu>
  )
})
