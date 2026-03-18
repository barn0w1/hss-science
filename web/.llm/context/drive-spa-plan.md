# Drive SPA — Implementation Plan

## Boilerplate Baseline

The project lives at `apps/drive-spa/` and already has:

- **React 19** + **TypeScript 5.9** (strict), **Vite 7**
- **Tailwind CSS v4** — configured via `@theme` blocks in `src/index.css` (no `tailwind.config.js`)
- **shadcn/ui** (style: `radix-vega`) — `components.json` present, only `button` installed
- **Lucide React** for icons
- **Inter Variable** font loaded
- `ThemeProvider` with dark/light/system, `d`-key toggle, localStorage persistence
- `cn()` utility in `src/lib/utils.ts`
- No router, no state manager, no MSW installed yet

---

## Packages to Add

```jsonc
// runtime
"zustand": "^5",
"react-router-dom": "^7",           // or tanstack/router — see routing section
"msw": "^2",
"@tanstack/react-query": "^5",       // server-state layer sitting on top of MSW

// DnD
"@dnd-kit/core": "^6",
"@dnd-kit/sortable": "^8",
"@dnd-kit/utilities": "^3",

// utilities
"date-fns": "^4",
"bytes": "^3",                       // human-readable file sizes
"mime": "^4"                         // mime type detection from extension
```

```jsonc
// dev
"@mswjs/data": "^0.16",              // in-memory DB for MSW
"@types/bytes": "^3"
```

---

## Domain Data Model

### Core Types

```typescript
// src/types/domain.ts

/** SHA-256 hex digest (64 chars) */
export type ContentHash = string & { readonly __brand: "ContentHash" };

/** Globally unique node identity */
export type NodeId = string & { readonly __brand: "NodeId" };

export type SpaceId = string & { readonly __brand: "SpaceId" };

export type UserId = string & { readonly __brand: "UserId" };

// ─── Node ────────────────────────────────────────────────────────────────────

export type NodeKind = "file" | "folder" | "symlink";

interface NodeBase {
  id: NodeId;
  spaceId: SpaceId;
  parentId: NodeId | null;  // null = root
  name: string;
  createdAt: string;        // ISO-8601
  updatedAt: string;
  createdBy: UserId;
}

export interface FileNode extends NodeBase {
  kind: "file";
  contentHash: ContentHash;
  size: number;             // bytes
  mimeType: string;
}

export interface FolderNode extends NodeBase {
  kind: "folder";
  childCount: number;       // denormalised for display
}

export interface SymlinkNode extends NodeBase {
  kind: "symlink";
  targetId: NodeId;
}

export type DriveNode = FileNode | FolderNode | SymlinkNode;

// ─── Space ───────────────────────────────────────────────────────────────────

export type SpaceRole = "owner" | "editor" | "viewer";

export interface SpaceMember {
  userId: UserId;
  role: SpaceRole;
  addedAt: string;
}

export interface Space {
  id: SpaceId;
  name: string;
  ownerId: UserId;
  personal: boolean;   // true = personal drive, one per user
  members: SpaceMember[];
  createdAt: string;
  rootNodeId: NodeId;  // always a FolderNode
}

// ─── Auth ────────────────────────────────────────────────────────────────────

export interface User {
  id: UserId;
  displayName: string;
  email: string;
  avatarUrl: string | null;
}

// ─── CAS Blob reference ──────────────────────────────────────────────────────

export interface BlobRef {
  hash: ContentHash;
  size: number;
  mimeType: string;
  /** Pre-signed URL (mock: /api/blobs/:hash) */
  url: string;
}

// ─── API Pagination ──────────────────────────────────────────────────────────

export interface Page<T> {
  items: T[];
  nextCursor: string | null;
  total: number;
}
```

### Tree Traversal Helpers

```typescript
// src/lib/tree.ts

/**
 * Build the ancestor chain from a node up to root.
 * Returns [root, …ancestors, node] order (breadcrumb-ready).
 */
export function buildBreadcrumb(
  nodeId: NodeId,
  nodesById: Map<NodeId, DriveNode>,
): DriveNode[] { /* traverse parentId chain */ }

/**
 * Flat list of all descendants (depth-first).
 */
export function flattenSubtree(
  rootId: NodeId,
  nodesById: Map<NodeId, DriveNode>,
): DriveNode[] { /* … */ }
```

---

## File / Folder Structure

```
src/
├── types/
│   ├── domain.ts          ← all domain types above
│   └── api.ts             ← request/response envelope types
│
├── lib/
│   ├── utils.ts           ← existing cn()
│   ├── tree.ts            ← breadcrumb, flatten helpers
│   ├── format.ts          ← formatBytes, formatDate, getMimeIcon
│   └── hash.ts            ← sha256 via SubtleCrypto (for upload preview)
│
├── mocks/
│   ├── db.ts              ← @mswjs/data model
│   ├── handlers/
│   │   ├── auth.ts
│   │   ├── spaces.ts
│   │   ├── nodes.ts
│   │   └── blobs.ts
│   ├── browser.ts         ← setupWorker(…handlers)
│   └── seed.ts            ← deterministic seed data
│
├── store/
│   ├── auth.store.ts
│   ├── drive.store.ts
│   └── ui.store.ts
│
├── hooks/
│   ├── useNodes.ts        ← react-query wrappers
│   ├── useSpaces.ts
│   ├── useDriveAction.ts  ← create / rename / move / delete
│   └── useUpload.ts       ← file → CAS hash → POST blob → POST node
│
├── components/
│   ├── ui/                ← shadcn-installed components
│   ├── layout/
│   │   ├── AppShell.tsx
│   │   ├── Sidebar.tsx
│   │   ├── SidebarSpaceSwitcher.tsx
│   │   ├── SidebarNav.tsx
│   │   └── TopBar.tsx
│   ├── drive/
│   │   ├── DriveView.tsx          ← route outlet
│   │   ├── NodeGrid.tsx           ← card grid layout
│   │   ├── NodeList.tsx           ← table layout
│   │   ├── NodeCard.tsx
│   │   ├── NodeRow.tsx
│   │   ├── NodeIcon.tsx           ← icon by mimeType / kind
│   │   ├── NodeContextMenu.tsx
│   │   ├── NodeRenameInput.tsx
│   │   ├── Breadcrumb.tsx
│   │   ├── ToolBar.tsx            ← view toggle, sort, search
│   │   ├── UploadDropzone.tsx
│   │   ├── UploadProgress.tsx
│   │   └── SelectionBar.tsx       ← bulk action bar
│   ├── preview/
│   │   ├── PreviewPanel.tsx       ← slide-in right panel
│   │   ├── ImagePreview.tsx
│   │   ├── TextPreview.tsx
│   │   └── GenericPreview.tsx
│   ├── dialogs/
│   │   ├── NewFolderDialog.tsx
│   │   ├── MoveDialog.tsx
│   │   ├── ShareDialog.tsx
│   │   └── DeleteConfirmDialog.tsx
│   └── theme-provider.tsx         ← existing
│
├── pages/
│   ├── DrivePage.tsx        ← /drive/:spaceId?/:nodeId?
│   ├── SharedPage.tsx       ← /shared
│   ├── TrashPage.tsx        ← /trash
│   └── NotFoundPage.tsx
│
├── App.tsx                  ← router root
├── main.tsx                 ← MSW bootstrap + StrictMode
└── index.css
```

---

## Routing

Use **React Router v7** (data router, loaders).

```typescript
// src/App.tsx
import { createBrowserRouter, RouterProvider } from "react-router-dom"
import { AppShell } from "@/components/layout/AppShell"
import { DrivePage } from "@/pages/DrivePage"
import { SharedPage } from "@/pages/SharedPage"
import { TrashPage } from "@/pages/TrashPage"
import { NotFoundPage } from "@/pages/NotFoundPage"

const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { index: true, loader: () => redirect("/drive") },
      {
        path: "drive/:spaceId?/:nodeId?",
        element: <DrivePage />,
      },
      { path: "shared", element: <SharedPage /> },
      { path: "trash",  element: <TrashPage />  },
      { path: "*",      element: <NotFoundPage /> },
    ],
  },
])

export default function App() {
  return <RouterProvider router={router} />
}
```

---

## MSW Setup

### In-memory DB (mswjs/data)

```typescript
// src/mocks/db.ts
import { factory, primaryKey, manyOf, nullable } from "@mswjs/data"

export const db = factory({
  user: {
    id:          primaryKey(String),
    displayName: String,
    email:       String,
    avatarUrl:   nullable(String),
  },
  space: {
    id:         primaryKey(String),
    name:       String,
    personal:   Boolean,
    ownerId:    String,
    rootNodeId: String,
    createdAt:  String,
  },
  node: {
    id:          primaryKey(String),
    spaceId:     String,
    parentId:    nullable(String),
    kind:        String,         // "file" | "folder" | "symlink"
    name:        String,
    contentHash: nullable(String),
    size:        Number,
    mimeType:    String,
    childCount:  Number,
    targetId:    nullable(String),
    createdAt:   String,
    updatedAt:   String,
    createdBy:   String,
    deleted:     Boolean,        // soft-delete / trash
  },
  blob: {
    hash:     primaryKey(String),
    size:     Number,
    mimeType: String,
    dataUrl:  String,            // base64 for images, raw text for text files
  },
})
```

### Seed Data

```typescript
// src/mocks/seed.ts
import { db } from "./db"
import { nanoid } from "nanoid"   // or crypto.randomUUID()

export function seedDatabase() {
  // ─── Users ───────────────────────────────────────────────────────────────
  const alice = db.user.create({
    id: "user-alice",
    displayName: "Alice Lambert",
    email: "alice@example.com",
    avatarUrl: null,
  })

  // ─── Personal Space ───────────────────────────────────────────────────────
  const rootId = nanoid()
  const space = db.space.create({
    id: "space-personal",
    name: "My Drive",
    personal: true,
    ownerId: alice.id,
    rootNodeId: rootId,
    createdAt: new Date().toISOString(),
  })

  db.node.create({
    id: rootId, spaceId: space.id, parentId: null,
    kind: "folder", name: "root", childCount: 3,
    contentHash: null, size: 0, mimeType: "", targetId: null,
    deleted: false,
    createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    createdBy: alice.id,
  })

  // ─── Sample Folders / Files ───────────────────────────────────────────────
  const docsFolderId = nanoid()
  db.node.create({
    id: docsFolderId, spaceId: space.id, parentId: rootId,
    kind: "folder", name: "Documents", childCount: 2,
    contentHash: null, size: 0, mimeType: "", targetId: null,
    deleted: false,
    createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
    createdBy: alice.id,
  })
  // … more seed nodes
}
```

### Mock Handlers

```typescript
// src/mocks/handlers/nodes.ts
import { http, HttpResponse } from "msw"
import { db } from "../db"

const BASE = "/api"

export const nodeHandlers = [

  // List children of a folder
  http.get(`${BASE}/spaces/:spaceId/nodes`, ({ params, request }) => {
    const url = new URL(request.url)
    const parentId = url.searchParams.get("parentId") ?? null
    const items = db.node.findMany({
      where: { spaceId: { equals: params.spaceId as string },
               parentId: { equals: parentId },
               deleted: { equals: false } },
    })
    return HttpResponse.json({ items, total: items.length, nextCursor: null })
  }),

  // Get a single node
  http.get(`${BASE}/nodes/:nodeId`, ({ params }) => {
    const node = db.node.findFirst({ where: { id: { equals: params.nodeId as string } } })
    if (!node) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(node)
  }),

  // Create node (folder or file)
  http.post(`${BASE}/spaces/:spaceId/nodes`, async ({ params, request }) => {
    const body = await request.json() as Partial<typeof db.node._type>
    const node = db.node.create({
      ...body,
      id: crypto.randomUUID(),
      spaceId: params.spaceId as string,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      deleted: false,
    })
    return HttpResponse.json(node, { status: 201 })
  }),

  // Rename / move (PATCH)
  http.patch(`${BASE}/nodes/:nodeId`, async ({ params, request }) => {
    const body = await request.json() as { name?: string; parentId?: string }
    const node = db.node.update({
      where: { id: { equals: params.nodeId as string } },
      data: { ...body, updatedAt: new Date().toISOString() },
    })
    return HttpResponse.json(node)
  }),

  // Soft-delete (trash)
  http.delete(`${BASE}/nodes/:nodeId`, ({ params }) => {
    db.node.update({
      where: { id: { equals: params.nodeId as string } },
      data: { deleted: true },
    })
    return new HttpResponse(null, { status: 204 })
  }),
]
```

```typescript
// src/mocks/handlers/spaces.ts
import { http, HttpResponse } from "msw"
import { db } from "../db"

export const spaceHandlers = [
  http.get("/api/spaces", () => {
    const items = db.space.getAll()
    return HttpResponse.json({ items })
  }),

  http.get("/api/spaces/:spaceId", ({ params }) => {
    const space = db.space.findFirst({ where: { id: { equals: params.spaceId as string } } })
    if (!space) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(space)
  }),
]
```

```typescript
// src/mocks/handlers/auth.ts
import { http, HttpResponse } from "msw"
import { db } from "../db"

export const authHandlers = [
  // Return the first user as "current user" (no real auth in mock)
  http.get("/api/me", () => {
    const user = db.user.findFirst({ where: { id: { equals: "user-alice" } } })
    return HttpResponse.json(user)
  }),
]
```

```typescript
// src/mocks/handlers/blobs.ts
import { http, HttpResponse } from "msw"
import { db } from "../db"

export const blobHandlers = [
  // Upload a blob (CAS: idempotent by hash)
  http.put("/api/blobs/:hash", async ({ params, request }) => {
    const existing = db.blob.findFirst({ where: { hash: { equals: params.hash as string } } })
    if (existing) return HttpResponse.json(existing, { status: 200 })

    const contentType = request.headers.get("content-type") ?? "application/octet-stream"
    const size = Number(request.headers.get("content-length") ?? 0)
    const blob = db.blob.create({
      hash: params.hash as string,
      mimeType: contentType,
      size,
      dataUrl: "",   // mock: skip actual storage
    })
    return HttpResponse.json(blob, { status: 201 })
  }),

  // Fetch a blob (returns redirect to data URL)
  http.get("/api/blobs/:hash", ({ params }) => {
    const blob = db.blob.findFirst({ where: { hash: { equals: params.hash as string } } })
    if (!blob) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(blob)
  }),
]
```

```typescript
// src/mocks/browser.ts
import { setupWorker } from "msw/browser"
import { authHandlers }  from "./handlers/auth"
import { spaceHandlers } from "./handlers/spaces"
import { nodeHandlers }  from "./handlers/nodes"
import { blobHandlers }  from "./handlers/blobs"

export const worker = setupWorker(
  ...authHandlers,
  ...spaceHandlers,
  ...nodeHandlers,
  ...blobHandlers,
)
```

```typescript
// src/main.tsx  (updated bootstrap)
async function enableMocking() {
  if (import.meta.env.DEV) {
    const { worker } = await import("./mocks/browser")
    const { seedDatabase } = await import("./mocks/seed")
    seedDatabase()
    return worker.start({ onUnhandledRequest: "bypass" })
  }
}

enableMocking().then(() => {
  createRoot(document.getElementById("root")!).render(/* … */)
})
```

---

## Zustand Stores

### Auth Store

```typescript
// src/store/auth.store.ts
import { create } from "zustand"
import type { User } from "@/types/domain"

interface AuthState {
  user: User | null
  isLoading: boolean
  setUser: (user: User) => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  setUser: (user) => set({ user, isLoading: false }),
}))
```

### Drive Store

```typescript
// src/store/drive.store.ts
import { create } from "zustand"
import type { SpaceId, NodeId, DriveNode } from "@/types/domain"

interface DriveState {
  // Navigation
  currentSpaceId: SpaceId | null
  currentNodeId: NodeId | null   // folder being viewed (null = root of space)

  // Selection (multi-select)
  selectedIds: Set<NodeId>
  lastSelectedId: NodeId | null  // for shift-click range

  // Clipboard
  clipboard: { mode: "copy" | "cut"; ids: Set<NodeId> } | null

  // Actions
  setCurrentSpace: (spaceId: SpaceId) => void
  navigateTo: (nodeId: NodeId | null) => void
  select: (id: NodeId, modifiers?: { shift?: boolean; meta?: boolean }) => void
  selectAll: (ids: NodeId[]) => void
  clearSelection: () => void
  copy: () => void
  cut: () => void
  clearClipboard: () => void
}

export const useDriveStore = create<DriveState>((set, get) => ({
  currentSpaceId: null,
  currentNodeId: null,
  selectedIds: new Set(),
  lastSelectedId: null,
  clipboard: null,

  setCurrentSpace: (spaceId) =>
    set({ currentSpaceId: spaceId, currentNodeId: null, selectedIds: new Set() }),

  navigateTo: (nodeId) =>
    set({ currentNodeId: nodeId, selectedIds: new Set() }),

  select: (id, { shift = false, meta = false } = {}) =>
    set((s) => {
      if (meta) {
        const next = new Set(s.selectedIds)
        next.has(id) ? next.delete(id) : next.add(id)
        return { selectedIds: next, lastSelectedId: id }
      }
      // shift-range selection handled at component level using lastSelectedId
      return { selectedIds: new Set([id]), lastSelectedId: id }
    }),

  selectAll: (ids) => set({ selectedIds: new Set(ids) }),
  clearSelection: () => set({ selectedIds: new Set(), lastSelectedId: null }),
  copy: () => set((s) => ({ clipboard: { mode: "copy", ids: new Set(s.selectedIds) } })),
  cut:  () => set((s) => ({ clipboard: { mode: "cut",  ids: new Set(s.selectedIds) } })),
  clearClipboard: () => set({ clipboard: null }),
}))
```

### UI Store

```typescript
// src/store/ui.store.ts
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

  setViewMode: (mode: ViewMode) => void
  setSort: (field: SortField, dir?: SortDir) => void
  setSearchQuery: (q: string) => void
  openPreview: (nodeId: NodeId) => void
  closePreview: () => void
  toggleSidebar: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      viewMode: "grid",
      sortField: "name",
      sortDir: "asc",
      searchQuery: "",
      previewNodeId: null,
      isSidebarCollapsed: false,

      setViewMode: (viewMode) => set({ viewMode }),

      setSort: (field, dir) =>
        set((s) => ({
          sortField: field,
          sortDir: dir ?? (s.sortField === field && s.sortDir === "asc" ? "desc" : "asc"),
        })),

      setSearchQuery: (searchQuery) => set({ searchQuery }),
      openPreview: (nodeId) => set({ previewNodeId: nodeId }),
      closePreview: () => set({ previewNodeId: null }),
      toggleSidebar: () => set((s) => ({ isSidebarCollapsed: !s.isSidebarCollapsed })),
    }),
    { name: "drive-ui-prefs", partialize: (s) => ({ viewMode: s.viewMode, sortField: s.sortField, sortDir: s.sortDir }) }
  )
)
```

---

## React Query Layer

```typescript
// src/hooks/useNodes.ts
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import type { DriveNode, NodeId, SpaceId, Page } from "@/types/domain"

const API = "/api"

async function fetchChildren(spaceId: SpaceId, parentId: NodeId | null): Promise<Page<DriveNode>> {
  const params = new URLSearchParams()
  if (parentId) params.set("parentId", parentId)
  const res = await fetch(`${API}/spaces/${spaceId}/nodes?${params}`)
  if (!res.ok) throw new Error("Failed to load nodes")
  return res.json()
}

export function useChildren(spaceId: SpaceId | null, parentId: NodeId | null) {
  return useQuery({
    queryKey: ["nodes", spaceId, parentId],
    queryFn: () => fetchChildren(spaceId!, parentId),
    enabled: !!spaceId,
    staleTime: 30_000,
  })
}

export function useCreateFolder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ spaceId, parentId, name }: { spaceId: SpaceId; parentId: NodeId | null; name: string }) => {
      const res = await fetch(`${API}/spaces/${spaceId}/nodes`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ kind: "folder", parentId, name, childCount: 0 }),
      })
      return res.json()
    },
    onSuccess: (_, vars) => qc.invalidateQueries({ queryKey: ["nodes", vars.spaceId, vars.parentId] }),
  })
}

export function useDeleteNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (nodeId: NodeId) => {
      await fetch(`${API}/nodes/${nodeId}`, { method: "DELETE" })
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["nodes"] }),
  })
}
```

---

## Upload Flow (CAS)

```typescript
// src/hooks/useUpload.ts
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useAuthStore } from "@/store/auth.store"
import { useDriveStore } from "@/store/drive.store"
import type { SpaceId, NodeId, ContentHash } from "@/types/domain"

async function sha256(buffer: ArrayBuffer): Promise<ContentHash> {
  const digest = await crypto.subtle.digest("SHA-256", buffer)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("") as ContentHash
}

async function uploadBlob(hash: ContentHash, file: File): Promise<void> {
  await fetch(`/api/blobs/${hash}`, {
    method: "PUT",
    headers: { "Content-Type": file.type, "Content-Length": String(file.size) },
    body: file,
  })
}

async function createFileNode(spaceId: SpaceId, parentId: NodeId | null, file: File, hash: ContentHash) {
  const res = await fetch(`/api/spaces/${spaceId}/nodes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      kind: "file",
      name: file.name,
      parentId,
      contentHash: hash,
      size: file.size,
      mimeType: file.type || "application/octet-stream",
    }),
  })
  return res.json()
}

export function useUpload() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ spaceId, parentId, files }: { spaceId: SpaceId; parentId: NodeId | null; files: File[] }) => {
      const results = []
      for (const file of files) {
        const buf = await file.arrayBuffer()
        const hash = await sha256(buf)
        await uploadBlob(hash, file)
        const node = await createFileNode(spaceId, parentId, file, hash)
        results.push(node)
      }
      return results
    },
    onSuccess: (_, vars) => qc.invalidateQueries({ queryKey: ["nodes", vars.spaceId, vars.parentId] }),
  })
}
```

---

## UI/UX Design System

### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Sidebar (240px)          │  Main Area                       │
│  ┌────────────────────┐   │  ┌──────────────────────────┐    │
│  │ Logo / Space Name  │   │  │ TopBar                   │    │
│  │ ─────────────────  │   │  │  Breadcrumb  Search  ⋯  │    │
│  │ My Drive           │   │  ├──────────────────────────┤    │
│  │ Shared with me     │   │  │ ToolBar                  │    │
│  │ Trash              │   │  │  + New  ▤ Grid/List Sort │    │
│  │ ─────────────────  │   │  ├──────────────────────────┤    │
│  │ SPACES             │   │  │                          │    │
│  │  ● My Drive        │   │  │  NodeGrid / NodeList     │    │
│  │  ○ Team Alpha      │   │  │                          │    │
│  └────────────────────┘   │  └──────────────────────────┘    │
│                           │              │                   │
│                           │  PreviewPanel (slide-in, 320px)  │
└──────────────────────────────────────────────────────────────┘
```

### Interaction Model

| Action | Trigger |
|--------|---------|
| Open folder | Single click (list), double-click (grid) |
| Select | Single click |
| Multi-select | Cmd/Ctrl+click, Shift+click range |
| Select all | Cmd+A |
| Open preview | Click file (list), single click (grid) |
| Rename | F2 or double-click name |
| Context menu | Right-click |
| Delete | Delete/Backspace key (with confirmation) |
| New folder | Cmd+Shift+N |
| Upload | Cmd+U, or drag-and-drop onto folder |
| Copy/Cut/Paste | Cmd+C / Cmd+X / Cmd+V |

### shadcn/ui Components to Install

```bash
npx shadcn add dialog
npx shadcn add dropdown-menu
npx shadcn add context-menu
npx shadcn add tooltip
npx shadcn add toast          # or sonner
npx shadcn add input
npx shadcn add scroll-area
npx shadcn add separator
npx shadcn add avatar
npx shadcn add badge
npx shadcn add progress
npx shadcn add skeleton
npx shadcn add breadcrumb
npx shadcn add sheet          # PreviewPanel
npx shadcn add command        # spotlight search / command palette
npx shadcn add popover
```

### Visual Refinements (Linear/Notion polish)

- **Hover states**: subtle background shift (`bg-accent/50`) with 100ms ease transition
- **Selection state**: `bg-primary/10` border ring in primary color
- **Drag ghost**: semi-transparent clone, badge with count if multi-select
- **Drop target**: `border-2 border-primary border-dashed bg-primary/5` on valid drop zones
- **Loading skeletons**: match exact card / row dimensions, animate with `animate-pulse`
- **Empty states**: centered illustration + CTA ("Upload files or create a folder")
- **Context menu**: keyboard-navigable, grouped with separators, icons per action
- **Breadcrumb overflow**: truncate middle segments, show `…` popover
- **File icons**: colored by category (image=purple, doc=blue, video=red, audio=green, archive=yellow, code=cyan)
- **Rename input**: inline within card/row, auto-selects filename without extension
- **Upload progress**: floating card (bottom-right), per-file progress bar, collapsible
- **Notification toasts**: Sonner, bottom-right, with undo action on delete

---

## Dark Mode Strategy

The existing `ThemeProvider` handles `dark` / `light` / `system` via a data attribute. Tailwind v4 `@custom-variant dark` is already wired. All color tokens are defined as CSS variables — no extra work needed. Just use semantic tokens (`bg-background`, `text-foreground`, `border`, etc.) and the dark theme is free.

---

## Keyboard Shortcuts (global)

Implement via a `useKeyboardShortcuts` hook that registers `keydown` listeners on `document` and dispatches actions to the stores.

| Key | Action |
|-----|--------|
| `/` | Focus search |
| `Cmd+K` | Open command palette |
| `Cmd+A` | Select all |
| `Delete` | Trash selected |
| `F2` | Rename focused |
| `Escape` | Clear selection / close preview / close dialog |
| `Enter` | Open selected (navigate folder / preview file) |
| `Arrow keys` | Move focus in grid/list |
| `Cmd+Z` | Undo (trash → restore) |

---

## Implementation Order

1. **Dependencies** — install Zustand, React Router, React Query, MSW, dnd-kit, date-fns, bytes, mime
2. **Types** — `src/types/domain.ts`, `src/types/api.ts`
3. **MSW** — db, seed, handlers, wire into `main.tsx`
4. **Stores** — auth, drive, ui
5. **React Query hooks** — useChildren, useSpaces, useCreateFolder, useDeleteNode, useUpload
6. **Install shadcn components** — all listed above
7. **AppShell + Sidebar** — layout skeleton, routing wired
8. **DriveView + NodeGrid/NodeList** — core browsing
9. **Breadcrumb + Navigation** — navigateTo, URL sync
10. **ToolBar** — view toggle, sort, search filter
11. **Context Menu + Dialogs** — rename, delete, new folder
12. **Drag and Drop** — dnd-kit for move
13. **Upload** — dropzone + CAS hash + progress UI
14. **Preview Panel** — slide-in sheet with file details
15. **Share Dialog** — space member management
16. **Command Palette** — Cmd+K spotlight
17. **Keyboard shortcuts** — global hook
18. **Polish pass** — animations, skeletons, empty states, responsive tweaks

---

## Notes & Decisions

- **No real auth**: mock returns a fixed user (`user-alice`). Auth store hydrates from `/api/me` on app mount.
- **URL shape**: `/drive/space-personal/node-abc123` — spaceId + nodeId in path. Enables deep linking and back-button navigation.
- **Optimistic updates**: React Query's `onMutate` for create/rename/delete to eliminate perceived latency.
- **Trash vs hard delete**: MSW uses `deleted: true` flag. TrashPage queries `deleted = true`. Permanent delete removes the record.
- **Copy/paste across spaces**: clipboard holds nodeIds; paste reads current spaceId from drive store and POSTs new nodes (server would handle deep copy — mock just creates shallow copies).
- **Symlink resolution**: `SymlinkNode.targetId` is resolved client-side before display; broken symlinks shown with a warning icon.
- **Infinite scroll vs pagination**: Use cursor-based infinite query (`useInfiniteQuery`) for large folders; show 50 items per page.
