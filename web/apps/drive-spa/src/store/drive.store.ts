import { create } from "zustand"
import type { SpaceId, NodeId, DriveNode, Space } from "@/types/domain"

interface LocalMutation {
  deleted: Set<NodeId>
  renamed: Map<NodeId, string>
  created: DriveNode[]
}

interface DriveState {
  currentSpaceId: SpaceId | null
  currentNodeId: NodeId | null
  spaces: Space[]
  selectedIds: Set<NodeId>
  lastSelectedId: NodeId | null
  clipboard: { mode: "copy" | "cut"; ids: Set<NodeId> } | null
  mutations: LocalMutation
  renamingNodeId: NodeId | null

  setSpaces: (spaces: Space[]) => void
  setCurrentSpace: (spaceId: SpaceId) => void
  navigateTo: (nodeId: NodeId | null) => void
  select: (
    id: NodeId,
    modifiers?: { shift?: boolean; meta?: boolean },
    orderedIds?: NodeId[]
  ) => void
  selectAll: (ids: NodeId[]) => void
  clearSelection: () => void
  copy: () => void
  cut: () => void
  clearClipboard: () => void
  deleteSelected: () => void
  renameNode: (id: NodeId, name: string) => void
  createFolder: (node: DriveNode) => void
  startRename: (id: NodeId) => void
  cancelRename: () => void
}

export const useDriveStore = create<DriveState>()((set, get) => ({
  currentSpaceId: null,
  currentNodeId: null,
  spaces: [],
  selectedIds: new Set(),
  lastSelectedId: null,
  clipboard: null,
  renamingNodeId: null,
  mutations: {
    deleted: new Set(),
    renamed: new Map(),
    created: [],
  },

  setSpaces: (spaces) => set({ spaces }),

  setCurrentSpace: (spaceId) =>
    set({
      currentSpaceId: spaceId,
      currentNodeId: null,
      selectedIds: new Set(),
      lastSelectedId: null,
    }),

  navigateTo: (nodeId) =>
    set({
      currentNodeId: nodeId,
      selectedIds: new Set(),
      lastSelectedId: null,
    }),

  select: (id, modifiers = {}, orderedIds = []) => {
    const { selectedIds, lastSelectedId } = get()
    if (modifiers.shift && lastSelectedId && orderedIds.length > 0) {
      const fromIdx = orderedIds.indexOf(lastSelectedId)
      const toIdx = orderedIds.indexOf(id)
      if (fromIdx !== -1 && toIdx !== -1) {
        const [start, end] =
          fromIdx < toIdx ? [fromIdx, toIdx] : [toIdx, fromIdx]
        const rangeIds = orderedIds.slice(start, end + 1)
        set({ selectedIds: new Set(rangeIds), lastSelectedId: id })
        return
      }
    }
    if (modifiers.meta) {
      const next = new Set(selectedIds)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      set({ selectedIds: next, lastSelectedId: id })
      return
    }
    set({ selectedIds: new Set([id]), lastSelectedId: id })
  },

  selectAll: (ids) => set({ selectedIds: new Set(ids) }),

  clearSelection: () => set({ selectedIds: new Set(), lastSelectedId: null }),

  copy: () => {
    const { selectedIds } = get()
    if (selectedIds.size > 0)
      set({ clipboard: { mode: "copy", ids: new Set(selectedIds) } })
  },

  cut: () => {
    const { selectedIds } = get()
    if (selectedIds.size > 0)
      set({ clipboard: { mode: "cut", ids: new Set(selectedIds) } })
  },

  clearClipboard: () => set({ clipboard: null }),

  deleteSelected: () => {
    const { selectedIds, mutations } = get()
    const next = new Set(mutations.deleted)
    selectedIds.forEach((id) => next.add(id))
    set({
      mutations: { ...mutations, deleted: next },
      selectedIds: new Set(),
      lastSelectedId: null,
    })
  },

  renameNode: (id, name) => {
    const { mutations } = get()
    const renamed = new Map(mutations.renamed)
    renamed.set(id, name)
    set({ mutations: { ...mutations, renamed }, renamingNodeId: null })
  },

  createFolder: (node) => {
    const { mutations } = get()
    set({ mutations: { ...mutations, created: [...mutations.created, node] } })
  },

  startRename: (id) => set({ renamingNodeId: id }),

  cancelRename: () => set({ renamingNodeId: null }),
}))
