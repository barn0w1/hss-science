import type { DriveNode, NodeId } from "@/types/domain"

export function buildBreadcrumb(
  nodeId: NodeId,
  nodesById: Map<NodeId, DriveNode>
): DriveNode[] {
  const chain: DriveNode[] = []
  let current = nodesById.get(nodeId)
  while (current) {
    chain.unshift(current)
    if (current.parentId === null) break
    current = nodesById.get(current.parentId)
  }
  return chain
}
