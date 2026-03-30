# Phase 2 — UI/UX, Error Handling, Logging, and Polish

## 1. Scope

| Area | What changes |
|---|---|
| shadcn/ui | Init in project; add Button, Input, Label, Card, Alert, Separator, Badge components |
| FlowMessages | Render form-level messages as `Alert`, node-level as inline text |
| FlowForm | Render inputs/buttons/labels with shadcn components; human-readable group names |
| AuthCard | New shared layout component for all auth-focused pages |
| Route pages | Apply AuthCard or page layout; add `meta()` for page titles |
| Error boundaries | Per-route ErrorBoundary on all rendered routes; improved root ErrorBoundary |
| Logging | pino singleton; `handleError` in entry.server.tsx; classify-point logging |
| Security headers | Add X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy |
| `return_to` | Propagate `flow.return_to` to cross-flow init links |

## 2. Out of scope

- react-hook-form integration (Kratos uses native HTML form submission)
- Client-side validation
- Dark mode toggle (system preference already works via CSS)
- Toast notifications
- TOTP / WebAuthn / Passkeys UI (FlowForm already renders these nodes safely)
- i18n
- AAL2 / privileged session step-up improvements beyond what Phase 1 already covers

---

## 3. Key decisions

### 3.1 FlowForm and shadcn Input/Button

The existing `FlowForm` renders Kratos `flow.ui.nodes` dynamically using plain HTML elements. The native `<form>` → Kratos submission chain must not be broken.

- **Hidden inputs** (`type === "hidden"`): stay as raw `<input>` — no visual change needed.
- **Submit inputs** (`type === "submit"`): replace with shadcn `<Button type="submit" name={...} value={...}>`. Label text comes from `node.meta.label?.text`.
- **All other inputs**: replace `<input>` with shadcn `<Input>`; add `<Label>` from shadcn. Wrap in a `<div className="flex flex-col gap-1.5">` field container. When `node.messages` is non-empty, add `aria-invalid` on `<Input>` and render inline error text below.
- **Groups**: use `<Separator />` and a human-readable heading above each non-default group.

### 3.2 FlowMessages variants

`FlowMessages` is used at two call sites:
- `ui.messages` (form-level) → render as shadcn `Alert` components
- `node.messages` (field-level, below each input) → render as small `<p>` text

Add a `variant` prop: `"alert"` (default, used for form-level) and `"field"` (used inside FlowForm for node errors).

### 3.3 Alert success variant

shadcn Alert ships with `"default"` and `"destructive"` variants. Kratos messages have `type: "info" | "success" | "error"`. Add a `"success"` variant to `alert.tsx` after installation.

Mapping:
- `"error"` → `variant="destructive"`
- `"success"` → `variant="success"` (new variant, green tones using `--border-success` / `--text-success` or via Tailwind semantic classes)
- `"info"` → `variant="default"`

### 3.4 AuthCard component

New component at `app/components/AuthCard.tsx`:
```tsx
interface AuthCardProps {
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}
```
Renders a full-viewport-height centered layout with a max-w-md Card. Used by: login, registration, recovery, verification, error pages.

Settings uses its own wider layout (not AuthCard).

### 3.5 pino logging — scope

Keep logging minimal; focus on signal over noise:
1. `app/lib/logger.server.ts` — pino singleton; redact `cookie`, `authorization` headers; level from `LOG_LEVEL` env var (default `"info"`).
2. `app/lib/errors.ts` — log at each classification branch (info for handled cases, warn/error for 502).
3. `app/lib/session.ts` — log unexpected errors that are re-thrown.
4. `entry.server.tsx` — export `handleError` that logs uncaught server errors; skip aborted requests.

No per-loader request child loggers (too verbose for this app's scale). If needed later, migrate to `@react-router/express` with `getLoadContext`.

### 3.6 Security headers

Add in `entry.server.tsx` inside `onShellReady` before resolving the response:
```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), camera=(), microphone=()
```
Skip CSP for now — React Router v7 nonce support requires additional configuration and is out of scope.

### 3.7 `return_to` propagation

Kratos embeds the original `return_to` value in the flow object. Access it as `flow.return_to` (type: `string | undefined`) — verified against SDK types (LoginFlow.return_to, RegistrationFlow.return_to, RecoveryFlow.return_to are all `string | undefined`).

In login component: pass `flow.return_to` to `initUrl("registration", ...)` and `initUrl("recovery", ...)`.
In registration component: pass `flow.return_to` to `initUrl("login", ...)`.

---

## 4. File changes summary

| File | Change type |
|---|---|
| `web/apps/accounts/package.json` | Add `pino` |
| `web/apps/accounts/components.json` | New (created by shadcn CLI) |
| `app/app.css` | Modified by shadcn CLI (CSS variables added) |
| `app/lib/utils.ts` | New (created by shadcn CLI — `cn()` helper) |
| `app/lib/logger.server.ts` | New |
| `app/lib/errors.ts` | Modified (add logging) |
| `app/lib/session.ts` | Modified (add logging) |
| `app/components/ui/` | New directory (shadcn component sources) |
| `app/components/AuthCard.tsx` | New |
| `app/components/FlowMessages.tsx` | Modified (variant prop, Alert/text rendering) |
| `app/components/FlowForm.tsx` | Modified (shadcn Input/Button/Label, group labels) |
| `app/entry.server.tsx` | Modified (handleError export, security headers) |
| `app/root.tsx` | Modified (ErrorBoundary uses Alert/Card) |
| `app/routes/login.tsx` | Modified (AuthCard layout, meta, ErrorBoundary, return_to) |
| `app/routes/registration.tsx` | Modified (AuthCard layout, meta, ErrorBoundary, return_to) |
| `app/routes/recovery.tsx` | Modified (AuthCard layout, meta, ErrorBoundary) |
| `app/routes/verification.tsx` | Modified (AuthCard layout, meta, ErrorBoundary) |
| `app/routes/settings.tsx` | Modified (page layout, meta, ErrorBoundary) |
| `app/routes/error.tsx` | Modified (AuthCard layout, meta) |

---

## 5. Todo list

### Phase A — shadcn/ui initialization

- [x] Run `pnpm dlx shadcn@latest init --preset nova --yes` from `web/apps/accounts/`
  - Verified `components.json` created with `framework: react-router`, Tailwind v4, alias `~/`
  - Verified `app/lib/utils.ts` created with `cn()` using `clsx` + `tailwind-merge`
  - Verified `app/app.css` updated with CSS variables
- [x] Add shadcn components: `button input label card alert separator badge`
  - Read all generated files in `app/components/ui/`
  - Verified `Alert` has `"default"` and `"destructive"` variants
- [x] Add `"success"` variant to `app/components/ui/alert.tsx`
- [x] Run `pnpm typecheck` — passes with no errors

---

### Phase B — Logging

- [x] Add `pino` to `web/apps/accounts/package.json` dependencies (pino v10 bundles types — no @types/pino needed)
- [x] Run `pnpm install --filter accounts` from `web/`
- [x] Create `app/lib/logger.server.ts` with pino singleton and `createRequestLogger`
- [x] Update `app/lib/errors.ts` — logging at each classification branch
- [x] Update `app/lib/session.ts` — logging for unexpected errors
- [x] Update `app/entry.server.tsx` — export `handleError` (using `HandleErrorFunction` type from react-router); add security headers

---

### Phase C — Component upgrades

- [x] Update `app/components/FlowMessages.tsx` — variant prop, Alert/text rendering
- [x] Update `app/components/FlowForm.tsx` — shadcn Input/Button/Label, GROUP_LABELS, Separator between groups
- [x] Create `app/components/AuthCard.tsx`
- [x] Run `pnpm typecheck` — passes with no errors

---

### Phase D — Route page UI + meta() + ErrorBoundary

- [x] `app/routes/login.tsx` — AuthCard, meta, ErrorBoundary, return_to propagation
- [x] `app/routes/registration.tsx` — AuthCard, meta, ErrorBoundary, return_to propagation
- [x] `app/routes/recovery.tsx` — AuthCard (state-driven), meta, ErrorBoundary
- [x] `app/routes/verification.tsx` — AuthCard (state-driven), meta, ErrorBoundary
- [x] `app/routes/settings.tsx` — page layout, meta, ErrorBoundary
- [x] `app/routes/error.tsx` — AuthCard, meta
- [x] `app/root.tsx` ErrorBoundary — Alert + Card layout
- [x] Run `pnpm typecheck` — passes with no errors

---

### Phase F — Final verification

- [x] `pnpm typecheck` passes with zero errors
- [x] `Alert` variant type includes `"success"` — verified
- [x] `meta()` return types match `Route.MetaDescriptors` for each route — verified
- [x] `handleError` uses `HandleErrorFunction` type from react-router — verified
- [x] `flow.return_to` access is type-safe (SDK types it as `string | undefined`) — verified
- [x] All new imports use `~/` alias prefix — verified
- [x] `logger.server.ts` named with `.server.ts` suffix — enforces SSR-only bundling boundary
- [x] No new `any` or unconstrained `unknown` assertions beyond existing patterns
