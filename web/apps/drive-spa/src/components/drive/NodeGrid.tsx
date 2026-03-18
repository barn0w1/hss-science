import type { DriveNode } from "@/types/domain"
import { NodeCard } from "./NodeCard"
import { useDriveStore } from "@/store/drive.store"

interface NodeGridProps {
  nodes: DriveNode[]
}

export function NodeGrid({ nodes }: NodeGridProps) {
  const selectedIds = useDriveStore((s) => s.selectedIds)
  const orderedIds = nodes.map((n) => n.id)

  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-1 p-2">
      {nodes.map((node) => (
        <NodeCard
          key={node.id}
          node={node}
          isSelected={selectedIds.has(node.id)}
          orderedIds={orderedIds}
        />
      ))}
    </div>
  )
}
