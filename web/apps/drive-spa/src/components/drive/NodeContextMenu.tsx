import type { ReactNode } from "react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  Pencil,
  Trash2,
  Copy,
  Scissors,
  Info,
  ExternalLink,
} from "lucide-react"
import type { DriveNode } from "@/types/domain"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"
import { useNavigate, useParams } from "react-router-dom"

interface NodeContextMenuProps {
  node: DriveNode
  children: ReactNode
}

export function NodeContextMenu({ node, children }: NodeContextMenuProps) {
  const select = useDriveStore((s) => s.select)
  const startRename = useDriveStore((s) => s.startRename)
  const navigateTo = useDriveStore((s) => s.navigateTo)
  const copy = useDriveStore((s) => s.copy)
  const cut = useDriveStore((s) => s.cut)
  const openDeleteDialog = useUIStore((s) => s.openDeleteDialog)
  const openPreview = useUIStore((s) => s.openPreview)
  const navigate = useNavigate()
  const params = useParams()

  function ensureSelected() {
    select(node.id)
  }

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent
        className="w-52"
        onCloseAutoFocus={(e) => e.preventDefault()}
      >
        {node.kind === "folder" ? (
          <ContextMenuItem
            onClick={() => {
              ensureSelected()
              navigateTo(node.id)
              navigate(`/drive/${params.spaceId}/${node.id}`)
            }}
          >
            <ExternalLink className="mr-2 h-4 w-4" />
            Open
          </ContextMenuItem>
        ) : (
          <ContextMenuItem
            onClick={() => {
              ensureSelected()
              openPreview(node.id)
            }}
          >
            <Info className="mr-2 h-4 w-4" />
            View info
          </ContextMenuItem>
        )}
        <ContextMenuSeparator />
        <ContextMenuItem
          onClick={() => {
            ensureSelected()
            startRename(node.id)
          }}
        >
          <Pencil className="mr-2 h-4 w-4" />
          Rename
        </ContextMenuItem>
        <ContextMenuItem
          onClick={() => {
            ensureSelected()
            copy()
          }}
        >
          <Copy className="mr-2 h-4 w-4" />
          Copy
        </ContextMenuItem>
        <ContextMenuItem
          onClick={() => {
            ensureSelected()
            cut()
          }}
        >
          <Scissors className="mr-2 h-4 w-4" />
          Cut
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem
          onClick={() => {
            ensureSelected()
            openDeleteDialog()
          }}
          className="text-destructive focus:text-destructive"
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Delete
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
