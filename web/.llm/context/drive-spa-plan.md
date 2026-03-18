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
"zustand": "^5",
"react-router-dom": "^7",
"msw": "^2"
```

---

## Domain Data Model

### Core Types

```typescript
// src/types/domain.ts

/** SHA-256 hex digest (64 chars) */
export type ContentHash = string & { readonly __brand: "ContentHash" }

export type NodeId = string & { readonly __brand: "NodeId" }
export type SpaceId = string & { readonly __brand: "SpaceId" }
export type UserId  = string & { readonly __brand: "UserId"  }

// ─── Node ────────────────────────────────────────────────────────────────────

interface NodeBase {
  id: NodeId
  spaceId: SpaceId
  parentId: NodeId | null   // null = space root
  name: string
  createdAt: string         // ISO-8601
  updatedAt: string
  createdBy: UserId
}

export interface FileNode extends NodeBase {
  kind: "file"
  contentHash: ContentHash
  size: number              // bytes
  mimeType: string
}

export interface FolderNode extends NodeBase {
  kind: "folder"
  childCount: number        // denormalised for display
}

export interface SymlinkNode extends NodeBase {
  kind: "symlink"
  targetId: NodeId
}

export type DriveNode = FileNode | FolderNode | SymlinkNode

// ─── Space ───────────────────────────────────────────────────────────────────

export type SpaceRole = "owner" | "editor" | "viewer"

export interface SpaceMember {
  userId: UserId
  role: SpaceRole
}

export interface Space {
  id: SpaceId
  name: string
  ownerId: UserId
  personal: boolean         // true = personal drive, one per user
  members: SpaceMember[]
  createdAt: string
  rootNodeId: NodeId        // always a FolderNode
}

// ─── Auth ────────────────────────────────────────────────────────────────────

export interface User {
  id: UserId
  displayName: string
  email: string
  avatarUrl: string | null
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
```

---

## File / Folder Structure

```
src/
├── types/
│   └── domain.ts              ← all domain types above
│
├── lib/
│   ├── utils.ts               ← existing cn()
│   ├── tree.ts                ← buildBreadcrumb helper
│   └── format.ts              ← formatBytes, formatDate, getMimeIcon
│
├── mocks/
│   ├── fixtures.ts            ← static typed fixture data
│   ├── handlers/
│   │   ├── auth.ts
│   │   ├── spaces.ts
│   │   └── nodes.ts
│   └── browser.ts             ← setupWorker(…handlers)
│
├── store/
│   ├── auth.store.ts
│   ├── drive.store.ts
│   └── ui.store.ts
│
├── components/
│   ├── ui/                    ← shadcn-installed components
│   ├── layout/
│   │   ├── AppShell.tsx
│   │   ├── Sidebar.tsx
│   │   ├── SidebarSpaceSwitcher.tsx
│   │   ├── SidebarNav.tsx
│   │   └── TopBar.tsx
│   ├── drive/
│   │   ├── DriveView.tsx      ← route outlet
│   │   ├── NodeGrid.tsx       ← card grid layout
│   │   ├── NodeList.tsx       ← table layout
│   │   ├── NodeCard.tsx
│   │   ├── NodeRow.tsx
│   │   ├── NodeIcon.tsx       ← icon by mimeType / kind
│   │   ├── NodeContextMenu.tsx
│   │   ├── NodeRenameInput.tsx
│   │   ├── Breadcrumb.tsx
│   │   ├── ToolBar.tsx        ← view toggle, sort, search
│   │   └── SelectionBar.tsx   ← bulk action bar
│   ├── preview/
│   │   ├── PreviewPanel.tsx   ← slide-in right panel
│   │   └── GenericPreview.tsx
│   ├── dialogs/
│   │   ├── NewFolderDialog.tsx
│   │   ├── ShareDialog.tsx
│   │   └── DeleteConfirmDialog.tsx
│   └── theme-provider.tsx     ← existing
│
├── pages/
│   ├── DrivePage.tsx          ← /drive/:spaceId?/:nodeId?
│   ├── TrashPage.tsx          ← /trash
│   └── NotFoundPage.tsx
│
├── App.tsx                    ← router root
├── main.tsx                   ← MSW bootstrap + StrictMode
└── index.css
```

---

## Routing

```typescript
// src/App.tsx
import { createBrowserRouter, RouterProvider, redirect } from "react-router-dom"
import { AppShell }     from "@/components/layout/AppShell"
import { DrivePage }    from "@/pages/DrivePage"
import { TrashPage }    from "@/pages/TrashPage"
import { NotFoundPage } from "@/pages/NotFoundPage"

const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { index: true, loader: () => redirect("/drive") },
      { path: "drive/:spaceId?/:nodeId?", element: <DrivePage /> },
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

MSW uses **static fixture data only** — no in-memory DB, no mutations persisted.

### Fixtures

```typescript
// src/mocks/fixtures.ts
import type { User, Space, DriveNode, SpaceId, NodeId, UserId, ContentHash } from "@/types/domain"

export const ALICE: User = {
  id: "user-alice" as UserId,
  displayName: "Alice Lambert",
  email: "alice@example.com",
  avatarUrl: null,
}

export const PERSONAL_SPACE: Space = {
  id: "space-personal" as SpaceId,
  name: "@personal",
  personal: true,
  ownerId: ALICE.id,
  members: [{ userId: ALICE.id, role: "owner" }],
  createdAt: "2025-01-01T00:00:00Z",
  rootNodeId: "node-root" as NodeId,
}

export const TEAM_SPACE: Space = {
  id: "space-team" as SpaceId,
  name: "Team Alpha",
  personal: false,
  ownerId: ALICE.id,
  members: [{ userId: ALICE.id, role: "owner" }],
  createdAt: "2025-03-01T00:00:00Z",
  rootNodeId: "node-team-root" as NodeId,
}

export const NODES: DriveNode[] = [
  // ── Personal space root ──────────────────────────────────────────────────
  {
    id: "node-root" as NodeId, kind: "folder",
    spaceId: "space-personal" as SpaceId, parentId: null,
    name: "root", childCount: 3,
    createdAt: "2025-01-01T00:00:00Z", updatedAt: "2025-01-01T00:00:00Z",
    createdBy: ALICE.id,
  },
  {
    id: "node-docs" as NodeId, kind: "folder",
    spaceId: "space-personal" as SpaceId, parentId: "node-root" as NodeId,
    name: "Documents", childCount: 2,
    createdAt: "2025-01-05T10:00:00Z", updatedAt: "2025-02-10T14:00:00Z",
    createdBy: ALICE.id,
  },
  {
    id: "node-imgs" as NodeId, kind: "folder",
    spaceId: "space-personal" as SpaceId, parentId: "node-root" as NodeId,
    name: "Images", childCount: 3,
    createdAt: "2025-01-06T10:00:00Z", updatedAt: "2025-01-06T10:00:00Z",
    createdBy: ALICE.id,
  },
  {
    id: "node-readme" as NodeId, kind: "file",
    spaceId: "space-personal" as SpaceId, parentId: "node-root" as NodeId,
    name: "README.md", mimeType: "text/markdown",
    contentHash: "abc123" as ContentHash, size: 2048,
    createdAt: "2025-01-02T08:00:00Z", updatedAt: "2025-01-02T08:00:00Z",
    createdBy: ALICE.id,
  },
  // ── Documents folder ─────────────────────────────────────────────────────
  {
    id: "node-spec" as NodeId, kind: "file",
    spaceId: "space-personal" as SpaceId, parentId: "node-docs" as NodeId,
    name: "spec.pdf", mimeType: "application/pdf",
    contentHash: "def456" as ContentHash, size: 512000,
    createdAt: "2025-02-01T09:00:00Z", updatedAt: "2025-02-01T09:00:00Z",
    createdBy: ALICE.id,
  },
  {
    id: "node-notes" as NodeId, kind: "file",
    spaceId: "space-personal" as SpaceId, parentId: "node-docs" as NodeId,
    name: "notes.txt", mimeType: "text/plain",
    contentHash: "ghi789" as ContentHash, size: 340,
    createdAt: "2025-02-10T14:00:00Z", updatedAt: "2025-02-10T14:00:00Z",
    createdBy: ALICE.id,
  },
  // ── Team space root ───────────────────────────────────────────────────────
  {
    id: "node-team-root" as NodeId, kind: "folder",
    spaceId: "space-team" as SpaceId, parentId: null,
    name: "root", childCount: 1,
    createdAt: "2025-03-01T00:00:00Z", updatedAt: "2025-03-01T00:00:00Z",
    createdBy: ALICE.id,
  },
  {
    id: "node-team-assets" as NodeId, kind: "folder",
    spaceId: "space-team" as SpaceId, parentId: "node-team-root" as NodeId,
    name: "Assets", childCount: 0,
    createdAt: "2025-03-05T11:00:00Z", updatedAt: "2025-03-05T11:00:00Z",
    createdBy: ALICE.id,
  },
]
```

### Mock Handlers

```typescript
// src/mocks/handlers/auth.ts
import { http, HttpResponse } from "msw"
import { ALICE } from "../fixtures"

export const authHandlers = [
  http.get("/api/me", () => HttpResponse.json(ALICE)),
]
```

```typescript
// src/mocks/handlers/spaces.ts
import { http, HttpResponse } from "msw"
import { PERSONAL_SPACE, TEAM_SPACE } from "../fixtures"

const ALL_SPACES = [PERSONAL_SPACE, TEAM_SPACE]

export const spaceHandlers = [
  http.get("/api/spaces", () =>
    HttpResponse.json({ items: ALL_SPACES }),
  ),
  http.get("/api/spaces/:spaceId", ({ params }) => {
    const space = ALL_SPACES.find((s) => s.id === params.spaceId)
    if (!space) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(space)
  }),
]
```

```typescript
// src/mocks/handlers/nodes.ts
import { http, HttpResponse } from "msw"
import { NODES } from "../fixtures"

export const nodeHandlers = [
  // List children of a folder
  http.get("/api/spaces/:spaceId/nodes", ({ params, request }) => {
    const url = new URL(request.url)
    const parentId = url.searchParams.get("parentId") ?? null
    const items = NODES.filter(
      (n) => n.spaceId === params.spaceId && n.parentId === parentId,
    )
    return HttpResponse.json({ items, total: items.length })
  }),

  // Get a single node
  http.get("/api/nodes/:nodeId", ({ params }) => {
    const node = NODES.find((n) => n.id === params.nodeId)
    if (!node) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(node)
  }),
]
```

```typescript
// src/mocks/browser.ts
import { setupWorker } from "msw/browser"
import { authHandlers }  from "./handlers/auth"
import { spaceHandlers } from "./handlers/spaces"
import { nodeHandlers }  from "./handlers/nodes"

export const worker = setupWorker(
  ...authHandlers,
  ...spaceHandlers,
  ...nodeHandlers,
)
```

```typescript
// src/main.tsx  (updated bootstrap)
async function enableMocking() {
  if (import.meta.env.DEV) {
    const { worker } = await import("./mocks/browser")
    return worker.start({ onUnhandledRequest: "bypass" })
  }
}

enableMocking().then(() => {
  createRoot(document.getElementById("root")!).render(/* … */)
})
```

---

## Zustand Stores

Stores define **types and initial state only**. Action implementations are left for the build phase.

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

export const useAuthStore = create<AuthState>()(() => ({
  user: null,
  isLoading: true,
  setUser: (_user) => { /* impl */ },
}))
```

### Drive Store

```typescript
// src/store/drive.store.ts
import { create } from "zustand"
import type { SpaceId, NodeId } from "@/types/domain"

interface DriveState {
  // Navigation
  currentSpaceId: SpaceId | null
  currentNodeId: NodeId | null        // folder being viewed; null = space root

  // Selection
  selectedIds: Set<NodeId>
  lastSelectedId: NodeId | null       // anchor for shift-click range

  // Clipboard
  clipboard: { mode: "copy" | "cut"; ids: Set<NodeId> } | null

  // Actions (signatures only — impl in build phase)
  setCurrentSpace: (spaceId: SpaceId) => void
  navigateTo: (nodeId: NodeId | null) => void
  select: (id: NodeId, modifiers?: { shift?: boolean; meta?: boolean }) => void
  selectAll: (ids: NodeId[]) => void
  clearSelection: () => void
  copy: () => void
  cut: () => void
  clearClipboard: () => void
}

export const useDriveStore = create<DriveState>()(() => ({
  currentSpaceId: null,
  currentNodeId: null,
  selectedIds: new Set(),
  lastSelectedId: null,
  clipboard: null,
  setCurrentSpace: (_spaceId) => { /* impl */ },
  navigateTo: (_nodeId) => { /* impl */ },
  select: (_id, _modifiers) => { /* impl */ },
  selectAll: (_ids) => { /* impl */ },
  clearSelection: () => { /* impl */ },
  copy: () => { /* impl */ },
  cut: () => { /* impl */ },
  clearClipboard: () => { /* impl */ },
}))
```

### UI Store

```typescript
// src/store/ui.store.ts
import { create } from "zustand"
import { persist } from "zustand/middleware"
import type { NodeId } from "@/types/domain"

export type ViewMode  = "grid" | "list"
export type SortField = "name" | "updatedAt" | "size" | "kind"
export type SortDir   = "asc" | "desc"

interface UIState {
  viewMode: ViewMode
  sortField: SortField
  sortDir: SortDir
  searchQuery: string
  previewNodeId: NodeId | null
  isSidebarCollapsed: boolean

  // Actions (signatures only — impl in build phase)
  setViewMode: (mode: ViewMode) => void
  setSort: (field: SortField, dir?: SortDir) => void
  setSearchQuery: (q: string) => void
  openPreview: (nodeId: NodeId) => void
  closePreview: () => void
  toggleSidebar: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    () => ({
      viewMode: "grid" as ViewMode,
      sortField: "name" as SortField,
      sortDir: "asc" as SortDir,
      searchQuery: "",
      previewNodeId: null,
      isSidebarCollapsed: false,
      setViewMode: (_mode) => { /* impl */ },
      setSort: (_field, _dir) => { /* impl */ },
      setSearchQuery: (_q) => { /* impl */ },
      openPreview: (_nodeId) => { /* impl */ },
      closePreview: () => { /* impl */ },
      toggleSidebar: () => { /* impl */ },
    }),
    {
      name: "drive-ui-prefs",
      partialize: (s) => ({ viewMode: s.viewMode, sortField: s.sortField, sortDir: s.sortDir }),
    },
  ),
)
```

---

## UI/UX Design System

### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Sidebar (240px)          │  Main Area                       │
│  ┌────────────────────┐   │  ┌──────────────────────────┐    │
│  │ Logo               │   │  │ TopBar                   │    │
│  │ ─────────────────  │   │  │  Breadcrumb  Search  ⋯  │    │
│  │ Trash              │   │  ├──────────────────────────┤    │
│  │ ─────────────────  │   │  │ ToolBar                  │    │
│  │ SPACES             │   │  │  + New  ▤ Grid/List Sort │    │
│  │  ● @personal       │   │  ├──────────────────────────┤    │
│  │  ○ Team Alpha      │   │  │                          │    │
│  │                    │   │  │  NodeGrid / NodeList     │    │
│  └────────────────────┘   │  │                          │    │
│                           │  └──────────────────────────┘    │
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
| Copy/Cut/Paste | Cmd+C / Cmd+X / Cmd+V |

### shadcn/ui Components to Install

```bash
npx shadcn add dialog
npx shadcn add dropdown-menu
npx shadcn add context-menu
npx shadcn add tooltip
npx shadcn add sonner
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
- **Selection state**: `bg-primary/10` ring in primary color
- **Loading skeletons**: match exact card / row dimensions, `animate-pulse`
- **Empty states**: centered illustration + CTA ("Drop files or create a folder")
- **Context menu**: keyboard-navigable, grouped with separators, icons per action
- **Breadcrumb overflow**: truncate middle segments, show `…` popover
- **File icons**: colored by category (image=purple, doc=blue, video=red, audio=green, archive=yellow, code=cyan)
- **Rename input**: inline within card/row, auto-selects filename without extension
- **Notification toasts**: Sonner, bottom-right

---

## Dark Mode Strategy

The existing `ThemeProvider` handles `dark` / `light` / `system` via a data attribute. Tailwind v4 `@custom-variant dark` is already wired. Use semantic tokens (`bg-background`, `text-foreground`, `border`, etc.) throughout — dark theme is free.

---

## Keyboard Shortcuts

Implement via a `useKeyboardShortcuts` hook registered on `document`.

| Key | Action |
|-----|--------|
| `/` | Focus search |
| `Cmd+K` | Open command palette |
| `Cmd+A` | Select all |
| `Delete` | Trash selected |
| `F2` | Rename focused node |
| `Escape` | Clear selection / close preview / close dialog |
| `Enter` | Open selected (navigate folder / preview file) |
| `Arrow keys` | Move focus in grid/list |

---

## Implementation Order

1. **Dependencies** — install zustand, react-router-dom, msw
2. **Types** — `src/types/domain.ts`
3. **MSW** — fixtures, handlers, wire into `main.tsx`
4. **Stores** — auth, drive, ui (types + initial state)
5. **Install shadcn components** — all listed above
6. **AppShell + Sidebar** — layout skeleton, routing wired
7. **DriveView + NodeGrid/NodeList** — core browsing from fixture data
8. **Breadcrumb + Navigation** — buildBreadcrumb, URL sync
9. **ToolBar** — view toggle, sort, client-side search filter
10. **Implement store actions** — navigateTo, select, clipboard
11. **Context Menu + Dialogs** — rename, delete, new folder (UI only)
12. **Preview Panel** — slide-in sheet with node metadata
13. **Share Dialog** — display space members
14. **Command Palette** — Cmd+K spotlight over fixture nodes
15. **Keyboard shortcuts** — global hook
16. **Polish pass** — animations, skeletons, empty states, responsive tweaks

---

## Todo List

### Phase 1 — Project Setup

- [ ] Install runtime dependencies: `zustand`, `react-router-dom`, `msw`
- [ ] Run `npx msw init public/` to copy the service worker file
- [ ] Install shadcn components: `dialog`, `dropdown-menu`, `context-menu`, `tooltip`, `sonner`, `input`, `scroll-area`, `separator`, `avatar`, `badge`, `progress`, `skeleton`, `breadcrumb`, `sheet`, `command`, `popover`

---

### Phase 2 — Types & Fixtures

- [ ] Create `src/types/domain.ts` with all branded types (`NodeId`, `SpaceId`, `UserId`, `ContentHash`) and interfaces (`FileNode`, `FolderNode`, `SymlinkNode`, `DriveNode`, `Space`, `SpaceMember`, `User`)
- [ ] Create `src/lib/format.ts` with `formatBytes(n)`, `formatDate(iso)`, `getMimeIcon(mimeType)` stubs
- [ ] Create `src/lib/tree.ts` with `buildBreadcrumb(nodeId, nodesById)` implementation
- [ ] Create `src/mocks/fixtures.ts` with `ALICE`, `PERSONAL_SPACE`, `TEAM_SPACE`, and `NODES` array (personal root, Documents folder + 2 files, Images folder, team root + Assets folder)
- [ ] Create `src/mocks/handlers/auth.ts` — `GET /api/me`
- [ ] Create `src/mocks/handlers/spaces.ts` — `GET /api/spaces`, `GET /api/spaces/:spaceId`
- [ ] Create `src/mocks/handlers/nodes.ts` — `GET /api/spaces/:spaceId/nodes`, `GET /api/nodes/:nodeId`
- [ ] Create `src/mocks/browser.ts` combining all handlers into `setupWorker`
- [ ] Update `src/main.tsx` to bootstrap MSW before rendering (`enableMocking().then(render)`)

---

### Phase 3 — Stores

- [ ] Create `src/store/auth.store.ts` — `AuthState` type + initial state (`user: null`, `isLoading: true`)
- [ ] Create `src/store/drive.store.ts` — `DriveState` type + initial state (navigation, selection, clipboard stubs)
- [ ] Create `src/store/ui.store.ts` — `UIState` type + initial state (viewMode, sort, search, previewNodeId, sidebarCollapsed) with `persist` middleware

---

### Phase 4 — Layout Shell & Routing

- [ ] Update `src/App.tsx` to use `createBrowserRouter` with `AppShell` as root layout, routes: `/drive/:spaceId?/:nodeId?`, `/trash`, `*`
- [ ] Create `src/components/layout/AppShell.tsx` — full-height flex container with `<Sidebar>` + `<Outlet>`, wires auth hydration on mount
- [ ] Create `src/components/layout/Sidebar.tsx` — fixed-width panel, renders `SidebarNav` + `SidebarSpaceSwitcher`
- [ ] Create `src/components/layout/SidebarNav.tsx` — Trash link; active state driven by `useLocation`
- [ ] Create `src/components/layout/SidebarSpaceSwitcher.tsx` — lists all spaces from `GET /api/spaces`; active space highlighted; navigates to space root on click
- [ ] Create `src/components/layout/TopBar.tsx` — breadcrumb slot (left), search input (centre), theme toggle + user avatar (right)
- [ ] Create `src/pages/DrivePage.tsx` — reads `:spaceId` and `:nodeId` from params, syncs into `drive.store`, renders `DriveView`
- [ ] Create `src/pages/TrashPage.tsx` — reads deleted nodes from store, renders `NodeList`
- [ ] Create `src/pages/NotFoundPage.tsx`

---

### Phase 5 — Drive View & Node Display

- [ ] Create `src/components/drive/DriveView.tsx` — fetches children via `GET /api/spaces/:spaceId/nodes?parentId=`, applies client-side sort + search filter from `ui.store`, renders `NodeGrid` or `NodeList` based on `viewMode`
- [ ] Create `src/components/drive/NodeIcon.tsx` — returns a coloured Lucide icon based on `kind` and `mimeType` (folder=FolderIcon, image=ImageIcon+purple, pdf=FileTextIcon+blue, video=VideoIcon+red, audio=MusicIcon+green, archive=ArchiveIcon+yellow, code=CodeIcon+cyan, default=FileIcon)
- [ ] Create `src/components/drive/NodeCard.tsx` — grid card: large icon, name (truncated), size/date line, selection ring, hover state
- [ ] Create `src/components/drive/NodeGrid.tsx` — responsive CSS grid of `NodeCard` items; handles click-to-select and double-click-to-open (folders) / click-to-preview (files)
- [ ] Create `src/components/drive/NodeRow.tsx` — list row: icon, name, kind badge, size, updated date, selection checkbox; hover state
- [ ] Create `src/components/drive/NodeList.tsx` — table layout of `NodeRow` items with sortable column headers; handles click interactions
- [ ] Create `src/components/drive/SelectionBar.tsx` — floating bar appears when `selectedIds.size > 0`; shows count + bulk actions (delete, copy, cut)

---

### Phase 6 — Navigation & Breadcrumb

- [ ] Implement `navigateTo` and `setCurrentSpace` actions in `drive.store`
- [ ] Create `src/components/drive/Breadcrumb.tsx` — builds ancestor chain via `buildBreadcrumb`; collapses middle segments into a `…` popover when overflow; each segment is a link that calls `navigateTo`
- [ ] Sync URL ↔ store: `DrivePage` reads params on mount + on param change, pushes to store; store actions call `router.navigate`

---

### Phase 7 — Toolbar & Filtering

- [ ] Create `src/components/drive/ToolBar.tsx` — "+ New" dropdown (New Folder), grid/list toggle, sort dropdown (name / updated / size / kind, asc/desc), search input wired to `ui.store.searchQuery`
- [ ] Implement client-side search filter in `DriveView` — filters node list by `name.includes(searchQuery)` (case-insensitive)
- [ ] Implement client-side sort in `DriveView` — sorts by `sortField` / `sortDir`, folders always before files

---

### Phase 8 — Store Actions

- [ ] Implement `select` in `drive.store` — single click replaces selection; Cmd+click toggles; Shift+click extends range from `lastSelectedId`
- [ ] Implement `selectAll` / `clearSelection` in `drive.store`
- [ ] Implement `copy` / `cut` / `clearClipboard` in `drive.store`
- [ ] Implement `setViewMode`, `setSort`, `setSearchQuery`, `openPreview`, `closePreview`, `toggleSidebar` in `ui.store`
- [ ] Implement `setUser` in `auth.store`; call it in `AppShell` after fetching `/api/me`

---

### Phase 9 — Context Menu & Dialogs

- [ ] Create `src/components/drive/NodeContextMenu.tsx` — wraps `ContextMenu` from shadcn; groups: Open / Rename / Copy / Cut / Delete / Properties; each item dispatches to store or opens a dialog
- [ ] Create `src/components/dialogs/NewFolderDialog.tsx` — dialog with a name input; on confirm, adds a `FolderNode` to a `localNodes` slice in `drive.store`
- [ ] Create `src/components/dialogs/DeleteConfirmDialog.tsx` — lists selected node names; on confirm, marks them `deleted: true` in store
- [ ] Create `src/components/drive/NodeRenameInput.tsx` — inline input replacing the node name; auto-selects name without extension on mount; commits on Enter/blur, cancels on Escape; updates node name in store

---

### Phase 10 — Preview Panel

- [ ] Create `src/components/preview/PreviewPanel.tsx` — `Sheet` (slide-in from right, 320px); shows node name, kind, size, mimeType, contentHash (truncated), createdAt, updatedAt, createdBy; close button and Escape dismiss
- [ ] Create `src/components/preview/GenericPreview.tsx` — large centred `NodeIcon` + metadata grid for non-previewable types
- [ ] Wire `openPreview` / `closePreview` — file click in `NodeGrid`/`NodeList` opens panel; folder click navigates instead

---

### Phase 11 — Share Dialog

- [ ] Create `src/components/dialogs/ShareDialog.tsx` — lists `space.members` with avatar, display name, email, and role badge (owner / editor / viewer); read-only (no mutations in mock)

---

### Phase 12 — Command Palette

- [ ] Create a `useCommandPaletteItems()` hook that builds a flat list of spaces + nodes from fixture data (label, icon, action)
- [ ] Wire `Command` (shadcn) into a modal triggered by `Cmd+K`; items navigate to spaces/nodes or trigger actions (New Folder, Trash); fuzzy filtering built into the `Command` component

---

### Phase 13 — Keyboard Shortcuts

- [ ] Create `src/hooks/useKeyboardShortcuts.ts` — registers `keydown` on `document`, calls store actions:
  - `/` → focus search input
  - `Cmd+K` → open command palette
  - `Cmd+A` → `selectAll` with current folder's node IDs
  - `Delete` / `Backspace` → open `DeleteConfirmDialog` if selection non-empty
  - `F2` → trigger rename on focused node
  - `Escape` → `clearSelection`, close preview, close any open dialog
  - `Enter` → open/navigate focused node
  - `ArrowUp` / `ArrowDown` / `ArrowLeft` / `ArrowRight` → move focus in grid/list
- [ ] Mount `useKeyboardShortcuts` in `AppShell`

---

### Phase 14 — Polish

- [ ] Add `animate-pulse` skeleton cards/rows in `NodeGrid`/`NodeList` shown while data is loading
- [ ] Add empty-state component in `DriveView` — centred icon + message + CTA when folder has no children
- [ ] Add `Sonner` toaster in `AppShell`; fire toast on delete (with undo action that restores nodes in store)
- [ ] Audit all interactive elements for 100ms hover transitions (`transition-colors duration-100`)
- [ ] Verify dark mode on every component — check contrast on muted text, borders, selection rings
- [ ] Make sidebar collapsible on mobile (`< 768px`): hide sidebar, show hamburger in TopBar that toggles it as an overlay
- [ ] Test keyboard navigation end-to-end: Tab order, Enter/Escape on all dialogs, arrow-key focus in grid and list views

---

## Notes & Decisions

- **No real auth**: mock always returns `user-alice`. Auth store hydrates from `GET /api/me` on mount.
- **URL shape**: `/drive/space-personal/node-docs` — spaceId + nodeId in path. Enables deep linking and back-button navigation.
- **Mutations are UI-only**: create/rename/delete operations update Zustand store state directly; no API calls are made. The fixture data is the read source; store state is the write surface.
- **Symlink resolution**: `SymlinkNode.targetId` resolved client-side before display; broken symlinks show a warning icon.
- **Trash**: nodes marked `deleted: true` in a local Zustand slice; `TrashPage` reads from that slice.
- **No "Shared with me"**: sharing is Space-level only (owner / editor / viewer on a Space). There is no per-file sharing and therefore no shared-with-me view. The sidebar lists Spaces only.
- **Personal space naming**: the personal space carries name `"@personal"` — it is not special-cased in the UI beyond the `personal: true` flag on `Space`. The sidebar renders it like any other space entry.
