import { Trash2, Copy, Scissors, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"

export function SelectionBar() {
  const selectedIds = useDriveStore((s) => s.selectedIds)
  const clearSelection = useDriveStore((s) => s.clearSelection)
  const copy = useDriveStore((s) => s.copy)
  const cut = useDriveStore((s) => s.cut)
  const openDeleteDialog = useUIStore((s) => s.openDeleteDialog)

  if (selectedIds.size === 0) return null

  return (
    <div className="flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 shadow-lg">
      <span className="text-sm font-medium">{selectedIds.size} selected</span>
      <div className="ml-2 flex items-center gap-1">
        <Button
          variant="ghost"
          size="sm"
          onClick={copy}
          className="h-7 gap-1.5 text-xs"
        >
          <Copy className="h-3.5 w-3.5" /> Copy
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={cut}
          className="h-7 gap-1.5 text-xs"
        >
          <Scissors className="h-3.5 w-3.5" /> Cut
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={openDeleteDialog}
          className="h-7 gap-1.5 text-xs text-destructive hover:text-destructive"
        >
          <Trash2 className="h-3.5 w-3.5" /> Delete
        </Button>
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={clearSelection}
        className="ml-auto"
      >
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}
