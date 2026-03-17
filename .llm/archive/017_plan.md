# Implementation Plan: myaccount-spa

**Target**: `web/apps/myaccount-spa/` — React SPA for `myaccount.hss-science.org`
**Model**: Google Account (`myaccount.google.com`) — unified account management interface
**Visual design**: Material Design 3
**Component system**: shadcn/ui `radix-nova` style (already initialised)
**API**: `myaccount-bff` REST API (OpenAPI spec at `api/openapi/myaccount/v1/myaccount.yaml`)

---

## 1. Goals

- Profile management (display name, avatar)
- Linked federated accounts (Google, GitHub) with unlink
- Active device session listing and revocation
- Auth guard: unauthenticated users redirected to OIDC login via BFF
- Full light/dark theme support (already wired)
- Type-safe API client generated from the OpenAPI spec

---

## 2. Architecture Overview

```
Browser
  │
  │  TypeScript / React 19 / Vite 7
  │
  ├── React Router v7  (SPA client-side routing)
  │
  ├── TanStack Query  (server-state cache, background refetch, optimistic updates)
  │
  ├── openapi-fetch client  ──▶  myaccount-bff  (:8080 in dev, same-origin in prod)
  │     type-safe, generated from myaccount.yaml
  │
  ├── shadcn/ui  (radix-nova preset, Tailwind v4)
  │
  ├── @hss/tokens  (workspace package — M3 design tokens)
  │
  └── sonner  (toast notifications)
```

### Request / response flow

```
App mount
  └── GET /api/v1/auth/me
        ├── { logged_in: true }  → render app
        └── { logged_in: false } → window.location.href = '/api/v1/auth/login'
                                   (BFF redirects to OIDC authorize)
```

---

## 3. New Dependencies

### Runtime

| Package | Purpose |
|---------|---------|
| `react-router-dom` ^7 | Client-side SPA routing |
| `@tanstack/react-query` | Server-state cache, `useQuery` / `useMutation`, background refetch |
| `openapi-fetch` | Type-safe HTTP client (wraps fetch) |
| `sonner` | Toast notifications (required by shadcn pattern) |

### Dev / build

| Package | Purpose |
|---------|---------|
| `openapi-typescript` | Generate TypeScript types from the OpenAPI YAML at build time |

### Workspace

| Package | Purpose |
|---------|---------|
| `@hss/tokens` (workspace:*) | M3 design tokens — imported as CSS in `index.css` |

---

## 4. File & Directory Structure

```
web/apps/myaccount-spa/
└── src/
    ├── api/
    │   ├── generated.ts          # ← auto-generated; do not edit by hand
    │   └── client.ts             # openapi-fetch client instance + CSRF header
    │
    ├── components/
    │   ├── ui/                   # shadcn primitive components (managed by CLI)
    │   │   ├── button.tsx        # existing
    │   │   ├── avatar.tsx
    │   │   ├── badge.tsx
    │   │   ├── card.tsx
    │   │   ├── dialog.tsx
    │   │   ├── alert-dialog.tsx
    │   │   ├── dropdown-menu.tsx
    │   │   ├── input.tsx
    │   │   ├── label.tsx
    │   │   ├── separator.tsx
    │   │   ├── skeleton.tsx
    │   │   ├── tooltip.tsx
    │   │   └── sonner.tsx
    │   │
    │   ├── layout/
    │   │   ├── AppShell.tsx      # root shell: sidebar + content area
    │   │   ├── NavSidebar.tsx    # left nav, user identity header, nav links
    │   │   └── TopBar.tsx        # mobile header with hamburger + user avatar
    │   │
    │   └── shared/
    │       ├── UserAvatar.tsx    # Avatar + AvatarFallback (initials from name/email)
    │       ├── ProviderIcon.tsx  # SVG icon selector for "google" | "github"
    │       └── PageHeader.tsx    # Section page heading + optional description
    │
    ├── context/
    │   └── AuthContext.tsx       # AuthProvider: me state + redirect guard
    │
    ├── hooks/
    │   ├── useProfile.ts         # GET + PATCH /api/v1/profile
    │   ├── useSessions.ts        # GET + DELETE /api/v1/sessions[/{id}]
    │   └── useProviders.ts       # GET + DELETE /api/v1/providers/{id}
    │
    ├── pages/
    │   ├── ProfilePage.tsx
    │   ├── LinkedAccountsPage.tsx
    │   ├── SecurityPage.tsx
    │   └── NotFoundPage.tsx
    │
    ├── lib/
    │   └── utils.ts              # existing (cn)
    │
    ├── routes.ts                 # route path constants
    ├── App.tsx                   # BrowserRouter + route tree + AuthProvider
    ├── main.tsx                  # existing
    └── index.css                 # existing — extend with @hss/tokens + M3 mappings
```

---

## 5. Code Generation Script

Add to `package.json` scripts:

```json
"generate:api": "openapi-typescript ../../../api/openapi/myaccount/v1/myaccount.yaml -o src/api/generated.ts"
```

Run before `dev` or `build`. Consider adding it as a `prebuild` hook or Vite plugin spawn.

Generated file path: `src/api/generated.ts` — included in `.gitignore` or committed (team preference).

---

## 6. API Client (`src/api/client.ts`)

```ts
import createClient from 'openapi-fetch'
import type { paths } from './generated'

export const api = createClient<paths>({ baseUrl: '/' })
```

**CSRF**: The BFF middleware checks `X-Requested-With: XMLHttpRequest` on all mutating
requests (POST/PATCH/DELETE). Set it as a default header on the client instance — no cookie
reading required:

```ts
export const api = createClient<paths>({
  baseUrl: '/',
  headers: { 'X-Requested-With': 'XMLHttpRequest' },
})
```

**Error handling**: openapi-fetch returns `{ data, error, response }`. Hooks should surface
`error.error` + `error.message` (see `Error` schema in OpenAPI spec) via `sonner` toasts.

---

## 7. Design Token Integration

### 7.1 Import tokens package in `index.css`

Add at the top of `src/index.css` (before the existing Tailwind import):

```css
@import "@hss/tokens";
```

This brings in M3 primitive, semantic, motion, shape, and typography tokens as Tailwind
`@theme` custom properties.

### 7.2 Add workspace package reference

In `web/apps/myaccount-spa/package.json` dependencies:

```json
"@hss/tokens": "workspace:*"
```

### 7.3 Map M3 semantic tokens → shadcn CSS variables

Extend the `:root {}` block in `index.css` to bridge the token systems. All source names are
`--md-sys-color-*` from `semantic.css` (light-mode `:root` block), which resolve to
`--md-ref-palette-*` entries from `primitive.css`.

```css
:root {
  /* Primary — primary-40: rgb(65 95 145) (blue-navy) */
  --primary:            var(--md-sys-color-primary);
  --primary-foreground: var(--md-sys-color-on-primary);

  /* Background & foreground */
  --background:         var(--md-sys-color-background);              /* neutral-99 */
  --foreground:         var(--md-sys-color-on-background);           /* neutral-10 */

  /* Card — surface-container-low sits just above the page surface */
  --card:               var(--md-sys-color-surface-container-low);   /* neutral-96 */
  --card-foreground:    var(--md-sys-color-on-surface);              /* neutral-10 */

  /* Muted backgrounds (chip, tag, secondary button fill) */
  --muted:              var(--md-sys-color-surface-container);       /* neutral-93 */
  --muted-foreground:   var(--md-sys-color-on-surface-variant);      /* neutral-variant-30 */

  /* Border & ring */
  --border:             var(--md-sys-color-outline-variant);         /* neutral-variant-80 */
  --input:              var(--md-sys-color-outline-variant);
  --ring:               var(--md-sys-color-outline);                 /* neutral-variant-50 */

  /* Error → destructive — error-40: rgb(186 26 26) */
  --destructive:        var(--md-sys-color-error);

  /* Shape — medium corner = 12px */
  --radius:             var(--md-sys-shape-corner-medium);
}
```

The `.dark` / `[data-theme="dark"]` selector in `semantic.css` already re-maps all
`--md-sys-color-*` tokens to their dark-mode palette values, so the shadcn variables above
automatically resolve correctly in dark mode with no additional overrides needed.

### 7.4 Typography: M3 type scale

Keep `Geist Variable` (already loaded via `@fontsource-variable/geist`) as the font for this
implementation. The actual token names in `typography.css` are `--md-ref-typeface-brand`
(`'Google Sans', 'Roboto'`) and `--md-ref-typeface-plain` (`'Roboto', system-ui`) — these are
not placeholder values but the spec is waiting on font loading to be set up. Do not change
fonts or add new font packages now.

The `@theme inline` `--font-sans` mapping in `index.css` stays as `'Geist Variable'`;
`--md-ref-typeface-brand` and `--md-ref-typeface-plain` are intentionally not wired yet.

For reference, the type-scale tokens follow `--md-sys-typescale-{scale}-{property}` naming,
e.g. `--md-sys-typescale-body-medium-size` (14px) and
`--md-sys-typescale-title-large-size` (22px).

### 7.5 Motion tokens

Motion token names follow `--md-sys-motion-duration-*` and `--md-sys-motion-easing-*`.

Two tiers matter for this UI:

| Use | Duration token | Easing token | Resolved value |
|-----|---------------|-------------|----------------|
| Interactive element state change (hover, focus, press) | `--md-sys-motion-duration-short4` | `--md-sys-motion-easing-standard` | 200ms `cubic-bezier(0.2, 0, 0, 1)` |
| Component open/close (Dialog, Sheet, Dropdown) | `--md-sys-motion-duration-medium1` | `--md-sys-motion-easing-emphasized-decel` | 250ms `cubic-bezier(0.05, 0.7, 0.1, 1)` |

Wire the interactive default into `@theme inline` for use as a Tailwind utility:

```css
--transition-standard: var(--md-sys-motion-duration-short4)
                        var(--md-sys-motion-easing-standard);
/* = 200ms cubic-bezier(0.2, 0, 0, 1) */
```

Note: `medium1 = 250ms`, `short4 = 200ms`. The plan previously mis-stated `medium1` as 200ms
— that was incorrect.

---

## 8. Routing (`src/App.tsx` + `src/routes.ts`)

### routes.ts

```ts
export const ROUTES = {
  ROOT: '/',
  PROFILE: '/profile',
  LINKED_ACCOUNTS: '/linked-accounts',
  SECURITY: '/security',
} as const
```

### App.tsx

```tsx
<QueryClientProvider client={queryClient}>
  <BrowserRouter>
    <AuthProvider>
      <AppShell>
        <Routes>
          <Route path="/" element={<Navigate to="/profile" replace />} />
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/linked-accounts" element={<LinkedAccountsPage />} />
          <Route path="/security" element={<SecurityPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </AppShell>
      <Toaster />       {/* sonner */}
    </AuthProvider>
  </BrowserRouter>
</QueryClientProvider>
```

### Vite dev proxy (`vite.config.ts`)

```ts
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
},
```

---

## 9. Auth Guard (`src/context/AuthContext.tsx`)

```tsx
const AuthContext = createContext<{ userId: string | null }>({ userId: null })

export function AuthProvider({ children }) {
  const [userId, setUserId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.GET('/api/v1/auth/me').then(({ data }) => {
      if (data?.logged_in) {
        setUserId(data.user_id ?? null)
      } else {
        window.location.href = '/api/v1/auth/login'
      }
      setLoading(false)
    })
  }, [])

  if (loading) return <AppLoadingSkeleton />
  return <AuthContext.Provider value={{ userId }}>{children}</AuthContext.Provider>
}
```

The loading state renders a full-page skeleton to avoid layout flash.

---

## 10. Layout System

### AppShell — two-column layout

```
┌─────────────────────────────────────────────────────┐
│  NavSidebar (260px)  │  main content (flex-1)        │
│                      │                               │
│  [User identity]     │  <Outlet / page content>      │
│  ───────────────     │                               │
│  ○ Personal info     │                               │
│  ○ Security          │                               │
│  ○ Linked accounts   │                               │
│                      │                               │
└──────────────────────┴───────────────────────────────┘
```

On mobile (`< md` breakpoint): sidebar collapses. TopBar shows hamburger + avatar. Nav slides
in as a Sheet (shadcn side panel).

**NavSidebar items:**

| Icon (lucide) | Label | Route |
|---------------|-------|-------|
| `UserRound` | Personal info | `/profile` |
| `ShieldCheck` | Security | `/security` |
| `Link` | Linked accounts | `/linked-accounts` |

Active link uses `bg-primary-container` (custom token) or `bg-accent` (shadcn).

### UserAvatar (shared)

Renders `Avatar` + `AvatarImage` (from `profile.picture`) + `AvatarFallback` (initials from
`given_name` + `family_name`, or first two chars of email).

### NavSidebar — user identity header

The user identity area at the top of the sidebar is a `DropdownMenu` triggered by clicking
`UserAvatar`. The menu contains a single "Sign out" item:

```
[UserAvatar]  Full Name
              email@example.com   ▾
```

On "Sign out" click:
1. Call `POST /api/v1/auth/logout` → `{ redirect_to: string }`
2. `window.location.href = data.redirect_to` (BFF handles OIDC end-session + cookie clear)

Add `dropdown-menu` to shadcn install list (see §12).

---

## 11. Pages

### 11.1 ProfilePage (`/profile`)

**Data**: `GET /api/v1/profile` → `Profile`

**Layout**:

```
┌─ Profile header ──────────────────────────────┐
│  [Avatar 80px]  Full Name                     │
│                 email@example.com  ✓ Verified  │
└───────────────────────────────────────────────┘

┌─ Card: Basic info ────────────────────────────┐
│  Name           Jane Doe            [Edit]    │
│  Email          jane@example.com              │
│  Member since   Jan 2024                      │
└───────────────────────────────────────────────┘

┌─ Card: Profile picture ───────────────────────┐
│  [Avatar 48px]  URL: https://...   [Edit]     │
│  (note if picture_is_local)                   │
└───────────────────────────────────────────────┘
```

**Edit name**: opens a Dialog with a single `Input` field + submit.
- Uses `PATCH /api/v1/profile` with `{ name }`.
- Validation: non-empty, max 100 chars.
- Success: `toast.success("Name updated")` via sonner.
- Error: `toast.error(...)`.

**Edit picture**: Dialog with `Input` for URL + preview thumbnail.

**Loading state**: `Skeleton` for all text fields.

**Components used**: `Card` (full composition), `Button`, `Dialog`, `Input`, `Badge` (for
email verification status), `Avatar`, `Skeleton`, `Separator`.

---

### 11.2 LinkedAccountsPage (`/linked-accounts`)

**Data**: `GET /api/v1/providers` → `FederatedProvider[]`

**Layout**:

```
┌─ Page heading ──────────────────────────────────┐
│  Linked accounts                                │
│  Sign in with your connected accounts           │
└─────────────────────────────────────────────────┘

┌─ Provider card ─────────────────────────────────┐
│  [Google icon]  Google                          │
│                 linked-email@gmail.com           │
│                 Last used: 3 days ago            │
│                                      [Unlink]   │
└─────────────────────────────────────────────────┘

[... repeat per provider ...]

[Empty state when list is empty]
```

**Unlink**: `AlertDialog` confirmation before `DELETE /api/v1/providers/{identityId}`.
- Confirmation message: "Remove Google? You won't be able to sign in with it."
- 409 conflict error (last provider) surfaced as `Alert` callout — "You must keep at least
  one sign-in method".

**Empty state**: `Empty` component — "No linked accounts" + description.

**ProviderIcon component**: returns inline SVG or `<img>` for `"google"` | `"github"`.
Sized at `size-5` (20px), no sizing classes on the element itself per shadcn icon rules.

**Loading state**: 2–3 `Skeleton` card rows.

**Components used**: `Card`, `Button` (variant="destructive" outline), `AlertDialog`,
`Alert`, `Empty`, `Badge`, `Skeleton`, `Separator`, `Tooltip` (for last-used date full format).

---

### 11.3 SecurityPage (`/security`)

**Data**: `GET /api/v1/sessions` → `Session[]`

**Layout**:

```
┌─ Page heading ──────────────────────────────────┐
│  Your devices                                   │
│  These devices have access to your account      │
└─────────────────────────────────────────────────┘

┌─ Session row ───────────────────────────────────┐
│  [Monitor icon]  Chrome on macOS   [This device]│
│                  192.168.1.1                     │
│                  Active now                      │
│                                  [Sign out]     │
└─────────────────────────────────────────────────┘

[... repeat per session ...]

┌─ Danger zone ───────────────────────────────────┐
│  Sign out of all other devices                  │
│  [Sign out all other devices]                   │
└─────────────────────────────────────────────────┘
```

**Revoke session**: `AlertDialog` before `DELETE /api/v1/sessions/{sessionId}`.
**Revoke all others**: `AlertDialog` before `DELETE /api/v1/sessions`.
**Current session**: `is_current: true` → show `Badge` "This device"; no revoke button.

**Device icon heuristic**: parse `device_name` for keywords (Mobile, iPhone, Android →
`Smartphone`; iPad/Tablet → `Tablet`; otherwise → `Monitor`) using lucide icons.

**Date formatting**: "Last active X days ago" — use `Intl.RelativeTimeFormat`.

**Loading state**: 3 `Skeleton` rows.

**Components used**: `Card`, `Button` (variant="destructive"), `AlertDialog`, `Badge`,
`Separator`, `Skeleton`, `Tooltip` (precise date on hover for relative timestamps).

---

### 11.4 NotFoundPage

Simple centered message: "Page not found" + link back to `/profile`.

---

## 12. shadcn Components to Install

Run (in `web/apps/myaccount-spa/`):

```bash
pnpm dlx shadcn@latest add avatar badge card dialog alert-dialog dropdown-menu input label \
  separator skeleton tooltip sonner
```

Verify each after install: check sub-component grouping (per composition.md rules), ensure
`AvatarFallback` is present, all `Dialog`/`AlertDialog` have `Title`.

---

## 13. Hook Design

All data hooks use TanStack Query. Query keys are string constants defined alongside each hook.

### useProfile

```ts
// Query key: ['profile']
// useQuery: GET /api/v1/profile → Profile
// useMutation: PATCH /api/v1/profile
//   - onMutate: optimistic update via queryClient.setQueryData(['profile'], ...)
//   - onError: rollback to snapshot
//   - onSettled: queryClient.invalidateQueries({ queryKey: ['profile'] })
// Returns: { profile, isPending, error, updateProfile }
```

### useSessions

```ts
// Query key: ['sessions']
// useQuery: GET /api/v1/sessions → Session[]
// useMutation (revokeSession): DELETE /api/v1/sessions/{sessionId}
//   - onSuccess: queryClient.invalidateQueries({ queryKey: ['sessions'] })
// useMutation (revokeAllOther): DELETE /api/v1/sessions
//   - onSuccess: queryClient.invalidateQueries({ queryKey: ['sessions'] })
// Returns: { sessions, isPending, error, revokeSession, revokeAllOther }
```

No optimistic update for revoke — wait for server confirmation before removing rows (avoids
accidentally hiding the current session on a failed request).

### useProviders

```ts
// Query key: ['providers']
// useQuery: GET /api/v1/providers → FederatedProvider[]
// useMutation (unlinkProvider): DELETE /api/v1/providers/{identityId}
//   - onSuccess: queryClient.invalidateQueries({ queryKey: ['providers'] })
//   - onError: if response.status === 409 → set local 'conflictError' state for inline Alert
// Returns: { providers, isPending, error, conflictError, unlinkProvider, clearConflict }
```

409 is not toasted — it is surfaced as an inline `Alert` in `LinkedAccountsPage` because it
requires explanatory UI ("you must keep at least one sign-in method"), not just a transient
notification.

---

## 14. Error Handling Patterns

| Scenario | UX |
|----------|-----|
| Unauthenticated (401 on any call) | Redirect to `/api/v1/auth/login` |
| Mutation error (4xx non-auth) | `toast.error(error.message)` via sonner |
| 409 Conflict (last provider) | Inline `Alert` callout in page |
| Network error | `toast.error("Network error. Please try again.")` |
| Loading state | `Skeleton` components matching layout |

---

## 15. Accessibility & Semantics

- All `Dialog`/`AlertDialog`/`Sheet` components must include `Title` (use `sr-only` if visually
  hidden per composition.md).
- `Avatar` always includes `AvatarFallback`.
- Semantic page headings: `<h1>` per page, `<h2>` per card section.
- Keyboard navigation: `Tab`/`Shift+Tab` through all interactive elements, `Enter`/`Space` on
  buttons. React Router links are natively keyboard-accessible.
- `Tooltip` on relative dates provides full `datetime` for screen readers.

---

## 16. M3 Visual Design Notes

| M3 concept | Implementation |
|-----------|---------------|
| **Color scheme** | `--primary` → `--md-sys-color-primary` (primary-40: rgb 65 95 145, blue-navy); surfaces = background/neutral-99, card/neutral-96 |
| **Shape** | `--radius` = `--md-sys-shape-corner-medium` (12px) for cards; `--md-sys-shape-corner-full` (9999px) for avatar and badge pill |
| **Elevation** | Cards use `shadow-sm` (subtle); no heavy shadows — M3 tonal elevation via surface-container tone |
| **Typography** | Body: `--md-sys-typescale-body-medium-size` (14px); Title: `--md-sys-typescale-title-large-size` (22px); Label: `--md-sys-typescale-label-medium-size` (12px) |
| **Motion** | Interactive transitions: `--md-sys-motion-duration-short4` (200ms) + `--md-sys-motion-easing-standard`; component open/close: `--md-sys-motion-duration-medium1` (250ms) + `--md-sys-motion-easing-emphasized-decel` |
| **Primary container** | Active nav item background: add CSS var `--primary-container: var(--md-sys-color-primary-container)` to `:root` in `index.css`; apply as `bg-[var(--primary-container)]` in NavSidebar only |

---

## 17. Implementation Order

1. **Package installation** — add runtime deps (`react-router-dom`, `@tanstack/react-query`, `openapi-fetch`, `sonner`), codegen dev dep, workspace token dep
2. **Codegen** — run `generate:api`, commit `src/api/generated.ts`
3. **API client** (`src/api/client.ts`) — openapi-fetch instance with `X-Requested-With` header
4. **Design token wiring** — `index.css` import + CSS variable remapping
5. **Install shadcn components** — batch install, verify composition
6. **QueryClientProvider** — wrap app root; configure `staleTime` / `retry` defaults
7. **AuthContext + auth guard** — `GET /api/v1/auth/me` → redirect
8. **AppShell + NavSidebar** — layout skeleton, routing, avatar DropdownMenu with Sign out
9. **ProfilePage** — `useProfile` (useQuery + useMutation), Dialog edit forms, loading skeletons
10. **LinkedAccountsPage** — `useProviders` (useQuery + useMutation), AlertDialog, inline Alert for 409, empty state
11. **SecurityPage** — `useSessions` (useQuery + useMutations), AlertDialog, device icons
12. **Responsive polish** — TopBar for mobile, Sheet-based nav drawer
13. **NotFoundPage**

---

## 18. Key Constraints & Rules (Non-Negotiable)

From `shadcn/SKILL.md` and rule files:

- **`className` for layout only** — never override component colors
- **No `space-x-*`/`space-y-*`** — always `flex` + `gap-*`
- **No raw colors** — all colors via semantic tokens (`bg-primary`, `text-muted-foreground`)
- **No `dark:` overrides** — token system handles both modes
- **`cn()` for conditional classes**
- **Items always inside Group** — `SelectItem` → `SelectGroup`, etc.
- **`Avatar` always has `AvatarFallback`**
- **Dialog/Sheet/Drawer always has `Title`**
- **Button loading** = `<Spinner data-icon="inline-start" />` + `disabled`, not `isLoading` prop
- **Icons in Button** use `data-icon="inline-start"` | `"inline-end"`, no sizing classes on icon
- **`size-*` shorthand** when width = height
- **`Skeleton`** for loading states, not custom `animate-pulse` divs
- **`Alert`** for callouts, not custom styled divs

## 20. Todo List

### Phase 0: Package Installation & Setup ✅
- [x] Add runtime deps in `package.json`: `react-router-dom`, `@tanstack/react-query`, `openapi-fetch`, `sonner`
- [x] Add dev dep in `package.json`: `openapi-typescript`
- [x] Add workspace dep in `package.json`: `@hss/tokens: workspace:*`
- [x] Add `generate:api` script to `package.json` (corrected path: `../../../api/...`)
- [x] Add vite dev proxy (`/api → localhost:8080`) to `vite.config.ts`
- [x] Run `pnpm install` from workspace root
- [x] **Build check**: `pnpm build` + `pnpm lint` ✓

### Phase 1: Codegen & API Client ✅
- [x] Run `generate:api` to produce `src/api/generated.ts`
- [x] Create `src/api/client.ts` (openapi-fetch instance with CSRF header)

### Phase 2: Design Token Wiring ✅
- [x] Add `@import "@hss/tokens"` at top of `src/index.css`
- [x] Replace `:root {}` vars with M3 semantic token mappings (§7.3)
- [x] Removed `.dark {}` block (semantic.css handles dark mode automatically)
- [x] Add `--primary-container` and `--transition-standard` custom vars

### Phase 3: shadcn Components ✅
- [x] Install: `avatar badge card dialog alert-dialog dropdown-menu input label separator skeleton tooltip sonner alert empty sheet`
- [x] Verified `AvatarFallback` present in `avatar.tsx`
- [x] Verified `DialogTitle` present in `dialog.tsx`
- [x] Verified `AlertDialogTitle` present in `alert-dialog.tsx`

### Phase 4: App Foundation ✅
- [x] Create `src/routes.ts` (route constants)
- [x] Create `src/context/AuthContext.tsx` (me check + redirect guard + `useAuth` hook)
- [x] Update `src/App.tsx` (QueryClientProvider + BrowserRouter + Routes + AuthProvider + TooltipProvider + Toaster)
- [x] **Build check**: `pnpm build` + `pnpm lint` ✓

### Phase 5: Shared Components ✅
- [x] Create `src/components/shared/UserAvatar.tsx`
- [x] Create `src/components/shared/ProviderIcon.tsx`
- [x] Create `src/components/shared/PageHeader.tsx`

### Phase 6: Layout ✅
- [x] Create `src/components/layout/AppShell.tsx`
- [x] Create `src/components/layout/NavSidebar.tsx` (with user identity DropdownMenu + logout)
- [x] Create `src/components/layout/TopBar.tsx` (mobile header + Sheet nav drawer)
- [x] **Build check**: `pnpm build` + `pnpm lint` ✓

### Phase 7: Data Hooks ✅
- [x] Create `src/hooks/useProfile.ts` (`useQuery` + `useMutation` with optimistic update)
- [x] Create `src/hooks/useSessions.ts` (`useQuery` + two `useMutation`s)
- [x] Create `src/hooks/useProviders.ts` (`useQuery` + `useMutation` with 409 handling)

### Phase 8: Pages ✅
- [x] Create `src/pages/ProfilePage.tsx` (avatar, basic info card, edit dialogs, skeletons)
- [x] Create `src/pages/LinkedAccountsPage.tsx` (provider cards, AlertDialog unlink, empty state, 409 Alert)
- [x] Create `src/pages/SecurityPage.tsx` (session rows, device icons, AlertDialog revoke, danger zone)
- [x] Create `src/pages/NotFoundPage.tsx`
- [x] **Build check**: `pnpm build` + `pnpm lint` ✓ (build: 476kB JS + 87kB CSS, 0 errors)

---

## 19. Decisions

| # | Topic | Decision |
|---|-------|----------|
| 1 | **Font** | Keep `Geist Variable`. No font changes until `@hss/tokens` typography tokens are finalised. |
| 2 | **Data fetching** | `@tanstack/react-query` — `useQuery` + `useMutation` across all hooks. Established now to avoid a migration later. |
| 3 | **Profile editing** | Dialog form only. No inline editing. Consistent with Google Account pattern and safer on mobile. |
| 4 | **CSRF** | `X-Requested-With: XMLHttpRequest` header on client instance. No cookie reading required. |
| 5 | **Add provider (v1)** | Out of scope. List + unlink only. No "Add another account" UI. |
| 6 | **Avatar upload (v1)** | URL input only. File upload deferred until object storage service is ready. |
| 7 | **Logout placement** | DropdownMenu triggered by `UserAvatar` in NavSidebar header. "Sign out" calls `POST /api/v1/auth/logout` → redirect to `response.redirect_to`. |
