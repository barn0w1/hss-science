import { useEffect } from "react"
import { useNavigate } from "react-router-dom"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"
import { NODES } from "@/mocks/fixtures"

export function useKeyboardShortcuts() {
  const clearSelection = useDriveStore((s) => s.clearSelection)
  const selectAll = useDriveStore((s) => s.selectAll)
  const selectedIds = useDriveStore((s) => s.selectedIds)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const currentNodeId = useDriveStore((s) => s.currentNodeId)
  const mutations = useDriveStore((s) => s.mutations)

  const closePreview = useUIStore((s) => s.closePreview)
  const previewNodeId = useUIStore((s) => s.previewNodeId)
  const openCommandPalette = useUIStore((s) => s.openCommandPalette)
  const openDeleteDialog = useUIStore((s) => s.openDeleteDialog)

  const navigate = useNavigate()

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      const target = e.target as HTMLElement
      const isInput =
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable

      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault()
        openCommandPalette()
        return
      }

      if (isInput) return

      if (e.key === "/") {
        e.preventDefault()
        ;(globalThis as { focusSearch?: () => void }).focusSearch?.()
        return
      }

      if ((e.metaKey || e.ctrlKey) && e.key === "a") {
        e.preventDefault()
        const allNodes = [...NODES, ...mutations.created].filter(
          (n) =>
            n.spaceId === currentSpaceId &&
            n.parentId === currentNodeId &&
            !mutations.deleted.has(n.id)
        )
        selectAll(allNodes.map((n) => n.id))
        return
      }

      if (e.key === "Escape") {
        if (previewNodeId) closePreview()
        else clearSelection()
        return
      }

      if (
        (e.key === "Delete" || e.key === "Backspace") &&
        selectedIds.size > 0
      ) {
        e.preventDefault()
        openDeleteDialog()
        return
      }

      if ((e.metaKey || e.ctrlKey) && e.key === "c") {
        useDriveStore.getState().copy()
        return
      }

      if ((e.metaKey || e.ctrlKey) && e.key === "x") {
        useDriveStore.getState().cut()
        return
      }
    }

    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [
    clearSelection,
    selectAll,
    selectedIds,
    currentSpaceId,
    currentNodeId,
    mutations,
    closePreview,
    previewNodeId,
    openCommandPalette,
    openDeleteDialog,
    navigate,
  ])
}
