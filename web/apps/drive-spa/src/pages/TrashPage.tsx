import { useDriveStore } from "@/store/drive.store"
import { NODES } from "@/mocks/fixtures"
import { NodeList } from "@/components/drive/NodeList"
import { TopBar } from "@/components/layout/TopBar"
import { Trash2 } from "lucide-react"

export function TrashPage() {
  const deleted = useDriveStore((s) => s.mutations.deleted)
  const trashedNodes = NODES.filter((n) => deleted.has(n.id))

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar />
      <div className="flex flex-1 flex-col overflow-auto p-4">
        <h2 className="mb-4 text-sm font-semibold text-muted-foreground uppercase tracking-wider">
          Trash
        </h2>
        {trashedNodes.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-3 text-center">
            <Trash2 className="h-12 w-12 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">Trash is empty</p>
          </div>
        ) : (
          <NodeList nodes={trashedNodes} />
        )}
      </div>
    </div>
  )
}
