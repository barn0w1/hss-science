import type { DriveNode } from "@/types/domain"
import { NodeRow } from "./NodeRow"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"
import { cn } from "@/lib/utils"
import { ChevronUp, ChevronDown } from "lucide-react"
import type { SortField } from "@/store/ui.store"

interface NodeListProps {
  nodes: DriveNode[]
  showHeader?: boolean
}

export function NodeList({ nodes, showHeader = true }: NodeListProps) {
  const selectedIds = useDriveStore((s) => s.selectedIds)
  const sortField = useUIStore((s) => s.sortField)
  const sortDir = useUIStore((s) => s.sortDir)
  const setSort = useUIStore((s) => s.setSort)
  const orderedIds = nodes.map((n) => n.id)

  const columns: { label: string; field: SortField; className: string }[] = [
    { label: "Name", field: "name", className: "flex-1" },
    {
      label: "Size",
      field: "size",
      className: "hidden w-20 text-right sm:block",
    },
    {
      label: "Modified",
      field: "updatedAt",
      className: "hidden w-32 text-right md:block",
    },
  ]

  return (
    <div className="flex flex-col gap-0.5">
      {showHeader && (
        <div className="flex items-center gap-3 px-3 pb-1">
          {columns.map(({ label, field, className }) => (
            <button
              key={field}
              onClick={() => setSort(field)}
              className={cn(
                "flex shrink-0 items-center gap-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground",
                className
              )}
            >
              {label}
              {sortField === field &&
                (sortDir === "asc" ? (
                  <ChevronUp className="h-3 w-3" />
                ) : (
                  <ChevronDown className="h-3 w-3" />
                ))}
            </button>
          ))}
        </div>
      )}
      {nodes.map((node) => (
        <NodeRow
          key={node.id}
          node={node}
          isSelected={selectedIds.has(node.id)}
          orderedIds={orderedIds}
        />
      ))}
    </div>
  )
}
