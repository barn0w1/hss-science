import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { useUIStore } from "@/store/ui.store"
import { useDriveStore } from "@/store/drive.store"
import type { NodeId, FolderNode, UserId } from "@/types/domain"
import { toast } from "sonner"

export function NewFolderDialog() {
  const isOpen = useUIStore((s) => s.isNewFolderDialogOpen)
  const closeNewFolderDialog = useUIStore((s) => s.closeNewFolderDialog)
  const createFolder = useDriveStore((s) => s.createFolder)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const currentNodeId = useDriveStore((s) => s.currentNodeId)

  const [name, setName] = useState("")

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || !currentSpaceId) return

    const folder: FolderNode = {
      id: `node-new-${Date.now()}` as NodeId,
      kind: "folder",
      spaceId: currentSpaceId,
      parentId: currentNodeId,
      name: trimmed,
      childCount: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      createdBy: "user-alice" as UserId,
    }
    createFolder(folder)
    toast.success(`Folder "${trimmed}" created`)
    setName("")
    closeNewFolderDialog()
  }

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => !open && closeNewFolderDialog()}
    >
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>New folder</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            autoFocus
            placeholder="Folder name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={closeNewFolderDialog}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim()}>
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
