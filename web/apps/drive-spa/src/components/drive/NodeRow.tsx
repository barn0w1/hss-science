import { memo } from "react"
import { cn } from "@/lib/utils"
import type { DriveNode } from "@/types/domain"
import { NodeIcon } from "./NodeIcon"
import { formatBytes, formatDate } from "@/lib/format"
import { NodeContextMenu } from "./NodeContextMenu"
import { NodeRenameInput } from "./NodeRenameInput"
import { useDriveStore } from "@/store/drive.store"
import { useNavigate, useParams } from "react-router-dom"
import { useUIStore } from "@/store/ui.store"
import { Badge } from "@/components/ui/badge"

interface NodeRowProps {
  node: DriveNode
  isSelected: boolean
  orderedIds: string[]
}

export const NodeRow = memo(function NodeRow({
  node,
  isSelected,
  orderedIds,
}: NodeRowProps) {
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
      navigate(`/drive/${params.spaceId}/${node.id}`)
    }
  }

  return (
    <NodeContextMenu node={node}>
      <div
        role="row"
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
          "group flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 transition-colors duration-100 outline-none select-none",
          "hover:bg-accent/60 focus-visible:ring-2 focus-visible:ring-ring",
          isSelected && "bg-primary/10 ring-1 ring-primary/40"
        )}
      >
        <NodeIcon node={node} size="sm" />
        <div className="flex min-w-0 flex-1 items-center gap-2">
          {isRenaming ? (
            <NodeRenameInput node={node} />
          ) : (
            <span className="truncate text-sm font-medium">{displayName}</span>
          )}
          <Badge
            variant="outline"
            className="hidden shrink-0 text-[10px] capitalize sm:inline-flex"
          >
            {node.kind}
          </Badge>
        </div>
        <span className="hidden w-20 shrink-0 text-right text-xs text-muted-foreground sm:block">
          {node.kind === "file" ? formatBytes(node.size) : "—"}
        </span>
        <span className="hidden w-32 shrink-0 text-right text-xs text-muted-foreground md:block">
          {formatDate(node.updatedAt)}
        </span>
      </div>
    </NodeContextMenu>
  )
})
