import { create } from "zustand"
import { persist } from "zustand/middleware"
import type { NodeId } from "@/types/domain"

export type ViewMode = "grid" | "list"
export type SortField = "name" | "updatedAt" | "size" | "kind"
export type SortDir = "asc" | "desc"

interface UIState {
  viewMode: ViewMode
  sortField: SortField
  sortDir: SortDir
  searchQuery: string
  previewNodeId: NodeId | null
  isSidebarCollapsed: boolean
  isCommandPaletteOpen: boolean
  isNewFolderDialogOpen: boolean
  isDeleteDialogOpen: boolean
  isShareDialogOpen: boolean

  setViewMode: (mode: ViewMode) => void
  setSort: (field: SortField, dir?: SortDir) => void
  setSearchQuery: (q: string) => void
  openPreview: (nodeId: NodeId) => void
  closePreview: () => void
  toggleSidebar: () => void
  openCommandPalette: () => void
  closeCommandPalette: () => void
  openNewFolderDialog: () => void
  closeNewFolderDialog: () => void
  openDeleteDialog: () => void
  closeDeleteDialog: () => void
  openShareDialog: () => void
  closeShareDialog: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      viewMode: "grid" as ViewMode,
      sortField: "name" as SortField,
      sortDir: "asc" as SortDir,
      searchQuery: "",
      previewNodeId: null,
      isSidebarCollapsed: false,
      isCommandPaletteOpen: false,
      isNewFolderDialogOpen: false,
      isDeleteDialogOpen: false,
      isShareDialogOpen: false,

      setViewMode: (viewMode) => set({ viewMode }),

      setSort: (field, dir) => {
        const { sortField, sortDir } = get()
        if (dir !== undefined) {
          set({ sortField: field, sortDir: dir })
        } else if (field === sortField) {
          set({ sortDir: sortDir === "asc" ? "desc" : "asc" })
        } else {
          set({ sortField: field, sortDir: "asc" })
        }
      },

      setSearchQuery: (searchQuery) => set({ searchQuery }),
      openPreview: (nodeId) => set({ previewNodeId: nodeId }),
      closePreview: () => set({ previewNodeId: null }),
      toggleSidebar: () =>
        set((s) => ({ isSidebarCollapsed: !s.isSidebarCollapsed })),
      openCommandPalette: () => set({ isCommandPaletteOpen: true }),
      closeCommandPalette: () => set({ isCommandPaletteOpen: false }),
      openNewFolderDialog: () => set({ isNewFolderDialogOpen: true }),
      closeNewFolderDialog: () => set({ isNewFolderDialogOpen: false }),
      openDeleteDialog: () => set({ isDeleteDialogOpen: true }),
      closeDeleteDialog: () => set({ isDeleteDialogOpen: false }),
      openShareDialog: () => set({ isShareDialogOpen: true }),
      closeShareDialog: () => set({ isShareDialogOpen: false }),
    }),
    {
      name: "drive-ui-prefs",
      partialize: (s) => ({
        viewMode: s.viewMode,
        sortField: s.sortField,
        sortDir: s.sortDir,
        isSidebarCollapsed: s.isSidebarCollapsed,
      }),
    }
  )
)
