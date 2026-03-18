import { useNavigate } from "react-router-dom"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { useUIStore } from "@/store/ui.store"
import { useDriveStore } from "@/store/drive.store"
import { NODES } from "@/mocks/fixtures"
import { NodeIcon } from "./NodeIcon"
import { FolderPlus, Trash2, User, Users } from "lucide-react"

export function CommandPalette() {
  const isOpen = useUIStore((s) => s.isCommandPaletteOpen)
  const closeCommandPalette = useUIStore((s) => s.closeCommandPalette)
  const openNewFolderDialog = useUIStore((s) => s.openNewFolderDialog)
  const spaces = useDriveStore((s) => s.spaces)
  const setCurrentSpace = useDriveStore((s) => s.setCurrentSpace)
  const navigateTo = useDriveStore((s) => s.navigateTo)
  const navigate = useNavigate()

  function close() {
    closeCommandPalette()
  }

  function goToSpace(spaceId: string) {
    setCurrentSpace(spaceId as Parameters<typeof setCurrentSpace>[0])
    navigate(`/drive/${spaceId}`)
    close()
  }

  function goToNode(spaceId: string, nodeId: string) {
    setCurrentSpace(spaceId as Parameters<typeof setCurrentSpace>[0])
    navigateTo(nodeId as Parameters<typeof navigateTo>[0])
    navigate(`/drive/${spaceId}/${nodeId}`)
    close()
  }

  return (
    <CommandDialog open={isOpen} onOpenChange={(open) => !open && close()}>
      <CommandInput placeholder="Search files, spaces, actions…" />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>

        <CommandGroup heading="Spaces">
          {spaces.map((space) => (
            <CommandItem key={space.id} onSelect={() => goToSpace(space.id)}>
              {space.personal ? (
                <User className="mr-2 h-4 w-4 text-muted-foreground" />
              ) : (
                <Users className="mr-2 h-4 w-4 text-muted-foreground" />
              )}
              {space.name}
            </CommandItem>
          ))}
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Files & Folders">
          {NODES.filter((n) => n.parentId !== null).map((node) => (
            <CommandItem
              key={node.id}
              onSelect={() =>
                goToNode(
                  node.spaceId,
                  node.kind === "folder"
                    ? node.id
                    : (node.parentId ?? node.spaceId)
                )
              }
            >
              <NodeIcon node={node} size="sm" className="mr-2" />
              {node.name}
            </CommandItem>
          ))}
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Actions">
          <CommandItem
            onSelect={() => {
              openNewFolderDialog()
              close()
            }}
          >
            <FolderPlus className="mr-2 h-4 w-4 text-muted-foreground" />
            New folder
          </CommandItem>
          <CommandItem
            onSelect={() => {
              navigate("/trash")
              close()
            }}
          >
            <Trash2 className="mr-2 h-4 w-4 text-muted-foreground" />
            Go to Trash
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
