import { useMemo } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useDriveStore } from "@/store/drive.store"
import { NODES } from "@/mocks/fixtures"
import { buildBreadcrumb } from "@/lib/tree"
import type { NodeId } from "@/types/domain"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { Home, MoreHorizontal } from "lucide-react"

const nodesById = new Map(NODES.map((n) => [n.id, n]))

export function DriveBreadcrumb() {
  const currentNodeId = useDriveStore((s) => s.currentNodeId)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const spaces = useDriveStore((s) => s.spaces)
  const navigateTo = useDriveStore((s) => s.navigateTo)
  const navigate = useNavigate()
  const params = useParams()

  const currentSpace = spaces.find((s) => s.id === currentSpaceId)

  const breadcrumb = useMemo(() => {
    if (!currentNodeId) return []
    return buildBreadcrumb(currentNodeId as NodeId, nodesById).filter(
      (n) => n.parentId !== null
    )
  }, [currentNodeId])

  function handleNavigate(nodeId: NodeId | null) {
    navigateTo(nodeId)
    if (nodeId) {
      navigate(`/drive/${params.spaceId}/${nodeId}`)
    } else {
      navigate(`/drive/${params.spaceId}`)
    }
  }

  const MAX_VISIBLE = 3
  const hasOverflow = breadcrumb.length > MAX_VISIBLE
  const visibleItems = hasOverflow ? breadcrumb.slice(-MAX_VISIBLE) : breadcrumb
  const hiddenItems = hasOverflow ? breadcrumb.slice(0, -MAX_VISIBLE) : []

  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink
            onClick={() => handleNavigate(null)}
            className="flex cursor-pointer items-center gap-1.5 text-sm"
          >
            <Home className="h-3.5 w-3.5" />
            {currentSpace?.name ?? "Drive"}
          </BreadcrumbLink>
        </BreadcrumbItem>

        {hasOverflow && (
          <>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <Popover>
                <PopoverTrigger asChild>
                  <button className="flex items-center text-muted-foreground hover:text-foreground">
                    <MoreHorizontal className="h-4 w-4" />
                  </button>
                </PopoverTrigger>
                <PopoverContent className="w-48 p-1">
                  {hiddenItems.map((node) => (
                    <button
                      key={node.id}
                      onClick={() => handleNavigate(node.id)}
                      className="w-full truncate rounded px-3 py-1.5 text-left text-sm hover:bg-accent"
                    >
                      {node.name}
                    </button>
                  ))}
                </PopoverContent>
              </Popover>
            </BreadcrumbItem>
          </>
        )}

        {visibleItems.map((node, i) => {
          const isLast = i === visibleItems.length - 1
          return (
            <span key={node.id} className="flex items-center gap-1.5">
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                {isLast ? (
                  <BreadcrumbPage className="max-w-40 truncate text-sm">
                    {node.name}
                  </BreadcrumbPage>
                ) : (
                  <BreadcrumbLink
                    onClick={() => handleNavigate(node.id)}
                    className="max-w-30 cursor-pointer truncate text-sm"
                  >
                    {node.name}
                  </BreadcrumbLink>
                )}
              </BreadcrumbItem>
            </span>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
