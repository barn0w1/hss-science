import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useUIStore } from "@/store/ui.store"
import { useDriveStore } from "@/store/drive.store"
import { NODES } from "@/mocks/fixtures"
import { toast } from "sonner"

export function DeleteConfirmDialog() {
  const isOpen = useUIStore((s) => s.isDeleteDialogOpen)
  const closeDeleteDialog = useUIStore((s) => s.closeDeleteDialog)
  const selectedIds = useDriveStore((s) => s.selectedIds)
  const deleteSelected = useDriveStore((s) => s.deleteSelected)
  const mutations = useDriveStore((s) => s.mutations)

  const allNodes = [...NODES, ...mutations.created]

  const selectedNodes = allNodes.filter((n) => selectedIds.has(n.id))

  function handleConfirm() {
    const count = selectedIds.size
    deleteSelected()
    toast.success(`Moved ${count} item${count !== 1 ? "s" : ""} to trash`)
    closeDeleteDialog()
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && closeDeleteDialog()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Move to trash?</DialogTitle>
          <DialogDescription>
            {selectedNodes.length === 1
              ? `"${selectedNodes[0].name}" will be moved to trash.`
              : `${selectedNodes.length} items will be moved to trash.`}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={closeDeleteDialog}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={handleConfirm}>
            Move to trash
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
