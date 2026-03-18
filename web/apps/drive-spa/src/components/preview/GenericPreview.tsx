import { NodeIcon } from "@/components/drive/NodeIcon"
import { formatBytes, formatDate } from "@/lib/format"
import type { DriveNode } from "@/types/domain"

interface GenericPreviewProps {
  node: DriveNode
}

export function GenericPreview({ node }: GenericPreviewProps) {
  return (
    <div className="flex flex-col items-center gap-4 py-6">
      <div className="flex h-20 w-20 items-center justify-center rounded-2xl bg-muted/60">
        <NodeIcon node={node} size="lg" />
      </div>
      <div className="w-full space-y-3 px-1">
        {node.kind === "file" && (
          <>
            <Row label="Size" value={formatBytes(node.size)} />
            <Row label="Type" value={node.mimeType} />
            <Row label="Hash" value={`${node.contentHash.slice(0, 12)}…`} />
          </>
        )}
        {node.kind === "symlink" && (
          <Row label="Target" value={node.targetId} />
        )}
        <Row label="Created" value={formatDate(node.createdAt)} />
        <Row label="Modified" value={formatDate(node.updatedAt)} />
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-2">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <span className="truncate text-right text-xs font-medium">{value}</span>
    </div>
  )
}
