# Ory Kratos Self-Service UI — Implementation Plan

## 1. Overview

### Tech stack

| Item | Value |
|---|---|
| Framework | React Router v7.13.2 (SSR, framework mode) |
| Kratos SDK | @ory/kratos-client-fetch@v26.2.0 |
| Language | TypeScript |
| Styling | Tailwind CSS v4 (already configured) |

### SSR browser flow pattern (applies to every flow)

```
1. Browser navigates to Kratos init: KRATOS_PUBLIC_URL/self-service/<flow>/browser
   → Kratos creates flow, sets anti-CSRF cookie, 303s to our UI: /login?flow=<id>

2. Our loader fetches: GET KRATOS_INTERNAL_URL/self-service/<flow>/flows?id=<id>
   (forwarding browser Cookie header)
   → Kratos returns flow object with ui.action, ui.method, ui.nodes, ui.messages

3. SSR renders HTML from flow.ui.nodes

4. Browser submits native HTML form to flow.ui.action (Kratos browser URL)
   → Browser cookies accompany request automatically

5a. Validation error → Kratos 303s to /login?flow=<id> (same or new id)
    → Loader re-fetches, re-renders with error messages

5b. Success → Kratos issues session cookie, 303s to configured return URL
```

### Environment variables

| Variable | Deployment value | Purpose |
|---|---|---|
| `KRATOS_INTERNAL_URL` | `http://kratos-public.identity.svc.cluster.local` | SSR → Kratos (internal) |
| `KRATOS_PUBLIC_URL` | `https://accounts.hss-science.org` | Browser → Kratos (external) |

Hardcode defaults matching the deployment values; allow override via `process.env`.

---

## 2. Package addition

Add to `web/apps/accounts/package.json` dependencies:

```json
"@ory/kratos-client-fetch": "^26.2.0"
```

---

## 3. File structure

```
web/apps/accounts/app/
  root.tsx                      — document shell
  routes.ts                     — route config
  entry.client.tsx              — browser hydration
  entry.server.tsx              — SSR response builder
  app.css                       — global styles (already exists)
  lib/
    kratos.ts                   — FrontendApi singleton, cookie helper, init URL builder
    session.ts                  — getSession() / requireSession()
    errors.ts                   — handleFlowError() for Kratos error classification
  components/
    FlowForm.tsx                — renders flow.ui (nodes, messages, form wrapper)
    FlowMessages.tsx            — renders UiText[] (flow-level and node-level)
  routes/
    _index.tsx                  — / → redirect based on session
    login.tsx                   — /login
    registration.tsx            — /registration
    logout.tsx                  — /logout → fetch logout token, redirect
    settings.tsx                — /settings (requires session)
    recovery.tsx                — /recovery
    verification.tsx            — /verification
    error.tsx                   — /error?id=<id>
```

---

## 4. Routing configuration (`app/routes.ts`)

```ts
import { type RouteConfig, route, index } from "@react-router/dev/routes";

export default [
  index("./routes/_index.tsx"),
  route("login",        "./routes/login.tsx"),
  route("registration", "./routes/registration.tsx"),
  route("logout",       "./routes/logout.tsx"),
  route("settings",     "./routes/settings.tsx"),
  route("recovery",     "./routes/recovery.tsx"),
  route("verification", "./routes/verification.tsx"),
  route("error",        "./routes/error.tsx"),
] satisfies RouteConfig;
```

---

## 5. Root and entry files

### `app/root.tsx`

Standard React Router v7 document shell. Required exports:
- `Layout` — wraps `children` in `<html>` with `<Meta>`, `<Links>`, `<Scripts>`, `<ScrollRestoration>`
- `default` — renders `<Outlet />`
- `ErrorBoundary` — root-level catch for unhandled errors

Import `app.css` as a side-effect import at the top. No per-route `links()` export needed given the single global stylesheet.

### `app/entry.client.tsx`

```tsx
import { hydrateRoot } from "react-dom/client";
import { HydratedRouter } from "react-router/dom";

hydrateRoot(document, <HydratedRouter />);
```

### `app/entry.server.tsx`

Standard Node SSR handler using `renderToPipeableStream` + `ServerRouter`. Boilerplate from the React Router v7 docs; no custom logic needed.

---

## 6. Shared library

### 6.1 `app/lib/kratos.ts`

**Exports:**

```ts
// FrontendApi singleton pointed at internal Kratos URL
export const frontend: FrontendApi

// External browser-facing Kratos URL
export const KRATOS_PUBLIC_URL: string  // = process.env.KRATOS_PUBLIC_URL ?? "https://accounts.hss-science.org"

// Extract Cookie header string from a Request
export function getCookie(request: Request): string | undefined

// Build the URL to initialize a browser flow
// e.g. initUrl("login") → "https://accounts.hss-science.org/self-service/login/browser"
// e.g. initUrl("login", "https://...") → appends ?return_to=<encoded>
export function initUrl(flow: string, returnTo?: string): string
```

**Implementation notes:**
- Instantiate `new FrontendApi(new Configuration({ basePath: KRATOS_INTERNAL_URL }))`
- `getCookie` reads `request.headers.get("cookie") ?? undefined`
- `initUrl` builds `${KRATOS_PUBLIC_URL}/self-service/${flow}/browser` and appends `?return_to=...` only if `returnTo` is provided

### 6.2 `app/lib/session.ts`

**Exports:**

```ts
// Returns the active session, or null if unauthenticated (401)
// Rethrows on 403 (AAL/privileged) and unexpected errors
export async function getSession(request: Request): Promise<Session | null>

// Like getSession, but throws redirect("/login") if session is null
export async function requireSession(request: Request): Promise<Session>
```

**Implementation notes:**
- Calls `frontend.toSession({ cookie: getCookie(request) })`
- Catches `ResponseError`: status 401 → return null; status 403 → re-throw (caller handles)
- `requireSession` wraps `getSession` and does `throw redirect("/login")` when null

### 6.3 `app/lib/errors.ts`

**Exports:**

```ts
// Classifies a Kratos SDK error and throws the appropriate redirect or data error.
// Never returns — always throws.
export async function handleFlowError(
  error: unknown,
  flowType: string,   // "login" | "registration" | "settings" | "recovery" | "verification"
  request: Request,
): Promise<never>
```

**Classification logic** (in order):

| Condition | Action |
|---|---|
| Not a `ResponseError` | Re-throw as-is |
| `error.id === "session_already_available"` | `throw redirect("/settings")` |
| status 410 or `error.id === "self_service_flow_expired"` | `throw redirect(initUrl(flowType))` |
| status 401 / `session_inactive` | `throw redirect("/login")` |
| status 403 / `session_refresh_required` | `throw redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent(request.url))` |
| status 422 / `browser_location_change_required` | Read `redirect_browser_to` from body; redirect only if present, otherwise throw 502 |
| Anything else | `throw data("Upstream error", { status: 502 })` |

Parse the error body with `error.response.clone().json()` (wrapped in try/catch for safety). Import `ErrorBrowserLocationChangeRequiredFromJSON` from the SDK for the 422 case.

---

## 7. Shared components

### 7.1 `app/components/FlowMessages.tsx`

```ts
interface FlowMessagesProps {
  messages?: UiText[];
}
```

Renders each `UiText` in a `<p>` or `<div>`. Use `message.type` (`"info"` | `"error"` | `"success"`) for conditional Tailwind classes (e.g., red text for errors, green for success). Render `message.text` as the content.

### 7.2 `app/components/FlowForm.tsx`

```ts
interface FlowFormProps {
  ui: UiContainer;
  submitLabel?: string;   // fallback label if no submit node present
}
```

**Renders:**

```html
<form method={ui.method} action={ui.action}>
  <!-- flow-level messages -->
  <FlowMessages messages={ui.messages} />

  <!-- nodes grouped by node.group -->
  <!-- all groups render in the same <form>; separate <section> or <div> per group -->

  <!-- for each node in each group: -->
  <!-- node.type === "input"  → <label> + <input> + <FlowMessages messages={node.messages} /> -->
  <!-- node.type === "text"   → <div>{node.attributes.text.text}</div> -->
  <!-- node.type === "img"    → <img src={node.attributes.src} /> -->
  <!-- node.type === "div"    → <div id/class/data-* ... /> -->
  <!-- node.type === "script" → <script ...attributes /> -->
  <!-- node.type === "a"      → <a ...attributes /> -->
</form>
```

**Key implementation details:**

1. **Input nodes**: spread all attributes from `UiNodeInputAttributes` onto `<input>`:
   - `name`, `type`, `value` (as `defaultValue` for non-hidden), `required`, `disabled`, `autocomplete`
   - Hidden inputs (`type === "hidden"`) render without a label — this covers `csrf_token` and `method`
   - Label text comes from `node.meta.label?.text`, not from `node.attributes`

2. **Method dispatch**: Kratos provides one submit button per group with `name="method"` and the group's method value. These are rendered as regular `<input type="submit">` or `<button type="submit">` nodes from the flow. The browser sends only the clicked button's value — no extra logic needed.

3. **Group ordering**: render `default` group first (contains `csrf_token`, shared fields), then other groups in iteration order. The `default` group's inputs are mostly hidden; no visible section header needed.

4. **Node key**: use `${node.group}-${node.attributes.name ?? index}` for React keys.

5. **No hardcoded field names** anywhere in this component or its callers.

---

## 8. Route implementations

### 8.1 `/` — Index (`routes/_index.tsx`)

**Loader only** (no component rendered):
1. `const session = await getSession(request)`
2. If session → `throw redirect("/settings")`
3. Otherwise → `throw redirect(initUrl("login"))`

---

### 8.2 `/login` — Login (`routes/login.tsx`)

**Loader:**
1. Read `flowId = url.searchParams.get("flow")`
2. If missing → `throw redirect(initUrl("login"))`
3. `const flow = await frontend.getLoginFlow({ id: flowId, cookie: getCookie(request) })`
4. Catch → `await handleFlowError(error, "login", request)`
5. Return `{ flow }`

**Component:**
- `<h1>Sign in</h1>`
- `<FlowForm ui={loaderData.flow.ui} />`
- `<a href={initUrl("registration")}>Create account</a>`
- `<a href={initUrl("recovery")}>Forgot password?</a>`

**No action** — form submits directly to `flow.ui.action` (Kratos browser URL).

---

### 8.3 `/registration` — Registration (`routes/registration.tsx`)

**Loader:**
1. Read `?flow=`
2. If missing → `throw redirect(initUrl("registration"))`
3. `const flow = await frontend.getRegistrationFlow({ id: flowId, cookie: getCookie(request) })`
4. Catch → `await handleFlowError(error, "registration", request)`
5. Return `{ flow }`

**Component:**
- `<h1>Create account</h1>`
- `<FlowForm ui={loaderData.flow.ui} />`
- `<a href={initUrl("login")}>Already have an account? Sign in</a>`

---

### 8.4 `/logout` — Logout (`routes/logout.tsx`)

**Loader only** (no component):
1. Try `const logoutFlow = await frontend.createBrowserLogoutFlow({ cookie: getCookie(request) })`
2. Catch `ResponseError` status 401 → `throw redirect("/login")`
3. `throw redirect(logoutFlow.logout_url)`

This route is the target of `<a href="/logout">` links. The browser navigates here, our loader fetches the one-time logout token from Kratos, then redirects the browser to `logout_url` to complete logout.

---

### 8.5 `/settings` — Settings (`routes/settings.tsx`)

**Loader:**
1. `const session = await requireSession(request)` — redirects to /login if unauthenticated
2. Read `?flow=`
3. If missing → `throw redirect(initUrl("settings"))`
4. Try `const flow = await frontend.getSettingsFlow({ id: flowId, cookie: getCookie(request) })`
5. Catch:
  - 403 / `session_refresh_required` → `throw redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent(request.url))`
  - Others → `await handleFlowError(error, "settings", request)`
6. Return `{ flow, session }`

**Component:**
- `<h1>Account settings</h1>`
- Display current user identity (email from `session.identity.traits`)
- If `flow.state === "success"` → show success banner ("Settings saved")
- `<FlowForm ui={loaderData.flow.ui} />`
- All settings groups (profile, password) render inside this one form. Each group gets a `<section>` with a heading. The `default` group's hidden inputs are included but not visually grouped.
- `<a href="/logout">Sign out</a>` — links to our `/logout` route

**No action** — form submits to `flow.ui.action`.

---

### 8.6 `/recovery` — Account Recovery (`routes/recovery.tsx`)

**Loader:**
1. Read `?flow=`
2. If missing → `throw redirect(initUrl("recovery"))`
3. `const flow = await frontend.getRecoveryFlow({ id: flowId, cookie: getCookie(request) })`
4. Catch → `await handleFlowError(error, "recovery", request)`
5. Return `{ flow }`

**Component — state-driven:**

```
flow.state === "choose_method":
  <h1>Recover your account</h1>
  <p>Enter your email address and we'll send a recovery code.</p>
  <FlowForm ui={flow.ui} />

flow.state === "sent_email":
  <h1>Check your email</h1>
  <p>A recovery code has been sent. Enter it below.</p>
  <FlowForm ui={flow.ui} />

flow.state === "passed_challenge":
  <p>Redirecting...</p>
  — Recovery success transitions into settings flow; final redirect honors `return_to` when configured

default (unexpected state):
  <p>Redirecting...</p>
```

The `FlowForm` component renders the correct inputs for each state automatically — `flow.ui.nodes` changes between states. No per-state field inspection needed.

**Messaging:** Render Kratos-provided `flow.ui.messages` verbatim (neutral by design in Kratos).

---

### 8.7 `/verification` — Email Verification (`routes/verification.tsx`)

**Loader:**
1. Read `?flow=`
2. If missing → `throw redirect(initUrl("verification"))`
3. `const flow = await frontend.getVerificationFlow({ id: flowId, cookie: getCookie(request) })`
4. Catch → `await handleFlowError(error, "verification", request)`
5. Return `{ flow }`

**Component — state-driven:**

```
flow.state === "choose_method":
  <h1>Verify your email</h1>
  <FlowForm ui={flow.ui} />

flow.state === "sent_email":
  <h1>Enter verification code</h1>
  <FlowForm ui={flow.ui} />

flow.state === "passed_challenge":
  <h1>Email verified</h1>
  <p>Your email address has been successfully verified.</p>
  <a href="/settings">Go to settings</a>
```

---

### 8.8 `/error` — Kratos Error Page (`routes/error.tsx`)

**Loader:**
1. Read `errorId = url.searchParams.get("id")`
2. If missing → return `{ flowError: null }`
3. Try `const flowError = await frontend.getFlowError({ id: errorId })`
4. Catch (fetch fails) → return `{ flowError: null }`
5. Return `{ flowError }`

**Component:**
- If `flowError` is null → generic "Something went wrong" with a link to `/`
- Otherwise → display `flowError.error` fields:
  - Cast `flowError.error` as `{ code?: number; message?: string; reason?: string }` at render time (SDK types this as `object`)
  - Show code, message, reason
  - Link to `/` or back to the relevant flow

**No `ErrorBoundary` needed** on this route — errors fetching the error detail are handled gracefully in the loader.

---

## 9. Error handling matrix

| Situation | Status | error.id | Handling |
|---|---|---|---|
| `?flow=` missing | — | — | `redirect(initUrl(flowType))` |
| Already authenticated while fetching login/registration flow | — | `session_already_available` | `redirect("/settings")` |
| Flow expired | 410 | `self_service_flow_expired` | `redirect(initUrl(flowType))` |
| No active session | 401 | `session_inactive` | `redirect("/login")` |
| Privileged session required | 403 | `session_refresh_required` | `redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent(request.url))` |
| Browser location change | 422 | `browser_location_change_required` | `redirect(redirect_browser_to)` when present, otherwise `throw data(...)` |
| Kratos system error | — | — | Kratos redirects browser to `/error?id=<id>` |
| Unexpected upstream error | 5xx | — | `throw data(...)` → route `ErrorBoundary` |
| Unexpected JS error | — | — | root `ErrorBoundary` |

All `ResponseError` parsing uses `error.response.clone().json()` wrapped in try/catch to guard against non-JSON bodies. Guard `redirect_browser_to` because it is optional in SDK types.

---

## 10. CSRF and cookie invariants

These must hold throughout the implementation:

1. Every SSR fetch to Kratos includes `cookie: getCookie(request)` — no exceptions.
2. `getCookie` always returns the full `Cookie` header string from the incoming browser request.
3. The `FlowForm` component never strips or omits any node — the `csrf_token` hidden input in the `default` group is rendered automatically.
4. Form `action` and `method` are taken verbatim from `flow.ui.action` and `flow.ui.method`.
5. Forms are plain HTML `<form>` elements (not React Router `<Form>`) — browser submits them directly to Kratos, cookies accompany automatically.

---

## 11. Implementation order

1. **Add dependency**: `@ory/kratos-client-fetch` in `package.json`; run `pnpm install`
2. **Entry files**: `entry.client.tsx`, `entry.server.tsx`
3. **Root**: `root.tsx` with document shell and root `ErrorBoundary`
4. **Lib layer** (no side effects, test-friendly):
   - `lib/kratos.ts`
   - `lib/session.ts`
   - `lib/errors.ts`
5. **Shared components**:
   - `components/FlowMessages.tsx`
   - `components/FlowForm.tsx`
6. **Routes** (in this order — simpler first):
   - `routes/_index.tsx`
   - `routes/error.tsx`
   - `routes/login.tsx`
   - `routes/registration.tsx`
   - `routes/logout.tsx`
   - `routes/settings.tsx`
   - `routes/recovery.tsx`
   - `routes/verification.tsx`
7. **Route config**: `routes.ts`
8. **Verify config**: confirm `react-router.config.ts` has `ssr: true` (already done)
9. **Type check**: `pnpm typecheck` (runs `react-router typegen && tsc`)

---

## 12. Out of scope (this phase)

- TOTP / WebAuthn / Passkeys / OIDC — do not implement, but `FlowForm` must render `text`, `img`, `script`, and `div` node types gracefully (no crashes) in case they appear
- AAL2 step-up flows
- i18n / message ID localization (render `message.text` as-is)
- Polished UI / design system
- Client-side validation
- Session token / native API flows

---

## 13. Todo list

### Phase 0 — Setup

- [x] Add `"@ory/kratos-client-fetch": "^26.2.0"` to `dependencies` in `web/apps/accounts/package.json`
- [x] Run `pnpm install` inside `web/apps/accounts/` to install the new dependency
- [x] Confirm `@ory/kratos-client-fetch` types are visible (e.g. `import type { FrontendApi } from "@ory/kratos-client-fetch"` resolves without error)

---

### Phase 1 — Boilerplate

- [x] Create `app/entry.client.tsx`
  - [x] Import `hydrateRoot` from `react-dom/client` and `HydratedRouter` from `react-router/dom`
  - [x] Call `hydrateRoot(document, <HydratedRouter />)`
- [x] Create `app/entry.server.tsx`
  - [x] Export default `handleRequest` function with signature `(request, responseStatusCode, responseHeaders, routerContext)`
  - [x] Use `renderToPipeableStream` + `ServerRouter` + `createReadableStreamFromReadable` to return a streaming `Response`
  - [x] Set `Content-Type: text/html` on the response
- [x] Create `app/root.tsx`
  - [x] Import `app.css` as a side-effect (`import "./app.css"`)
  - [x] Export `Layout({ children })` — wraps `children` in `<html>` with `<Meta />`, `<Links />`, `<Scripts />`, `<ScrollRestoration />`
  - [x] Export default `App` component — renders `<Outlet />`
  - [x] Export `ErrorBoundary` — handles root-level uncaught errors; branch on `isRouteErrorResponse` for HTTP errors vs unexpected exceptions; provide a link back to `/`
- [x] Create `app/routes.ts`
  - [x] Import `route` and `index` from `@react-router/dev/routes`
  - [x] Register all 8 route modules: `_index`, `login`, `registration`, `logout`, `settings`, `recovery`, `verification`, `error`

---

### Phase 2 — Shared library

#### `app/lib/kratos.ts`

- [x] Declare `KRATOS_INTERNAL_URL` constant — `process.env.KRATOS_INTERNAL_URL ?? "http://kratos-public.identity.svc.cluster.local"`
- [x] Declare `KRATOS_PUBLIC_URL` constant — `process.env.KRATOS_PUBLIC_URL ?? "https://accounts.hss-science.org"`
- [x] Instantiate and export `frontend: FrontendApi` — `new FrontendApi(new Configuration({ basePath: KRATOS_INTERNAL_URL }))`
- [x] Export `getCookie(request: Request): string | undefined` — returns `request.headers.get("cookie") ?? undefined`
- [x] Export `initUrl(flow: string, returnTo?: string): string`
  - [x] Base: `${KRATOS_PUBLIC_URL}/self-service/${flow}/browser`
  - [x] Append `?return_to=<encodeURIComponent(returnTo)>` only when `returnTo` is provided

#### `app/lib/session.ts`

- [x] Export `getSession(request: Request): Promise<Session | null>`
  - [x] Call `frontend.toSession({ cookie: getCookie(request) })`
  - [x] Catch `ResponseError` — return `null` on status 401; rethrow on all other statuses
  - [x] Rethrow non-`ResponseError` errors unchanged
- [x] Export `requireSession(request: Request): Promise<Session>`
  - [x] Call `getSession(request)`
  - [x] If result is `null`, `throw redirect("/login")`
  - [x] Otherwise return the session

#### `app/lib/errors.ts`

- [x] Export `handleFlowError(error: unknown, flowType: string, request: Request): Promise<never>`
  - [x] If `error` is not a `ResponseError` — rethrow as-is
  - [x] Parse body: `const body = await error.response.clone().json().catch(() => undefined)`
  - [x] Extract `errorId`: `body?.error?.id as string | undefined`
  - [x] **Case `session_already_available`**: `throw redirect("/settings")`
  - [x] **Case 410 / `self_service_flow_expired`**: `throw redirect(initUrl(flowType))`
  - [x] **Case 401 / `session_inactive`**: `throw redirect("/login")`
  - [x] **Case 403 / `session_refresh_required`**: `throw redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent(request.url))`
  - [x] **Case 422 / `browser_location_change_required`**: parse body with `ErrorBrowserLocationChangeRequiredFromJSON`; if `redirectBrowserTo` is present `throw redirect(redirectBrowserTo)`, otherwise fall through to default
  - [x] **Default**: `throw data("Upstream error", { status: 502 })`

---

### Phase 3 — Shared components

#### `app/components/FlowMessages.tsx`

- [x] Define props interface: `{ messages?: UiText[] }`
- [x] Return `null` when `messages` is empty or undefined
- [x] Render each `UiText` in a `<p>` element
- [x] Apply a Tailwind class based on `message.type`: `"error"` → red text, `"success"` → green text, `"info"` → default/muted
- [x] Render `message.text` as the text content (no HTML injection)

#### `app/components/FlowForm.tsx`

- [x] Define props interface: `{ ui: UiContainer; submitLabel?: string }`
- [x] Render outer `<form action={ui.action} method={ui.method}>`
- [x] Render `<FlowMessages messages={ui.messages} />` at the top of the form (flow-level messages)
- [x] Group `ui.nodes` by `node.group` (preserving Kratos iteration order within each group)
- [x] For each group render a wrapping `<section>` or `<div>`; omit a visible heading for the `"default"` group
- [x] For each node dispatch by `node.type`:
  - [x] **`"input"`**:
    - [x] If `node.attributes.type !== "hidden"`: render `<label>` with text from `node.meta.label?.text`
    - [x] Render `<input>` with: `name`, `type`, `required`, `disabled`, `autoComplete`, `defaultValue` (from `node.attributes.value` for non-hidden; `value` for hidden)
    - [x] Render `<FlowMessages messages={node.messages} />` beneath the input
  - [x] **`"text"`**: render `<div>` with `(node.attributes as UiNodeTextAttributes).text.text`
  - [x] **`"img"`**: render `<img src={...} />` using `UiNodeImageAttributes`
  - [x] **`"div"`**: render a `<div>` passing through any id/class attributes — no crash if attributes are sparse
  - [x] **`"script"`**: render `<script>` with `src`, `integrity`, `crossOrigin`, `async` from `UiNodeScriptAttributes`
  - [x] **`"a"`**: render `<a>` with `href` and label text from `UiNodeAnchorAttributes`
  - [x] **Unknown type**: render nothing (no crash)
- [x] Use `${node.group}-${node.attributes.name ?? index}` as the React `key` for each node
- [x] Do not hardcode any field names anywhere in this component

---

### Phase 4 — Routes

#### `app/routes/_index.tsx`

- [x] Loader: call `getSession(request)`
- [x] If session is active → `throw redirect("/settings")`
- [x] Otherwise → `throw redirect(initUrl("login"))`
- [x] Export a minimal default component (or none — loader always redirects)

#### `app/routes/error.tsx`

- [x] Loader:
  - [x] Read `errorId = new URL(request.url).searchParams.get("id")`
  - [x] If missing → return `{ flowError: null }`
  - [x] Try `frontend.getFlowError({ id: errorId })`; catch all errors and return `{ flowError: null }`
  - [x] Return `{ flowError }`
- [x] Component:
  - [x] If `flowError` is null → render generic "Something went wrong" message
  - [x] Otherwise cast `flowError.error` as `{ code?: number; message?: string; reason?: string }` and display each field that is present
  - [x] Render a link back to `/`

#### `app/routes/login.tsx`

- [x] Loader:
  - [x] Read `flowId = url.searchParams.get("flow")`
  - [x] If missing → `throw redirect(initUrl("login"))`
  - [x] Call `frontend.getLoginFlow({ id: flowId, cookie: getCookie(request) })`
  - [x] Catch → `await handleFlowError(error, "login", request)`
  - [x] Return `{ flow }`
- [x] Component:
  - [x] Render `<h1>Sign in</h1>`
  - [x] Render `<FlowForm ui={loaderData.flow.ui} />`
  - [x] Render `<a href={initUrl("registration")}>Create account</a>`
  - [x] Render `<a href={initUrl("recovery")}>Forgot password?</a>`

#### `app/routes/registration.tsx`

- [x] Loader:
  - [x] Read `flowId` from `?flow=`
  - [x] If missing → `throw redirect(initUrl("registration"))`
  - [x] Call `frontend.getRegistrationFlow({ id: flowId, cookie: getCookie(request) })`
  - [x] Catch → `await handleFlowError(error, "registration", request)`
  - [x] Return `{ flow }`
- [x] Component:
  - [x] Render `<h1>Create account</h1>`
  - [x] Render `<FlowForm ui={loaderData.flow.ui} />`
  - [x] Render `<a href={initUrl("login")}>Already have an account? Sign in</a>`

#### `app/routes/logout.tsx`

- [x] Loader only (no component):
  - [x] Call `frontend.createBrowserLogoutFlow({ cookie: getCookie(request) })`
  - [x] Catch `ResponseError` with status 401 → `throw redirect("/login")`
  - [x] `throw redirect(logoutFlow.logout_url)`

#### `app/routes/settings.tsx`

- [x] Loader:
  - [x] Call `requireSession(request)` — will redirect to `/login` automatically if unauthenticated
  - [x] Read `flowId` from `?flow=`
  - [x] If missing → `throw redirect(initUrl("settings"))`
  - [x] Call `frontend.getSettingsFlow({ id: flowId, cookie: getCookie(request) })`
  - [x] Catch:
    - [x] `ResponseError` status 403 with `error.id === "session_refresh_required"` → `throw redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent(request.url))`
    - [x] Others → `await handleFlowError(error, "settings", request)`
  - [x] Return `{ flow, session }`
- [x] Component:
  - [x] Render `<h1>Account settings</h1>`
  - [x] Display current user email from `session.identity.traits` (cast as `{ email?: string }`)
  - [x] If `flow.state === "success"` → render a success banner ("Settings saved")
  - [x] Render `<FlowForm ui={loaderData.flow.ui} />`
  - [x] Render `<a href="/logout">Sign out</a>`

#### `app/routes/recovery.tsx`

- [x] Loader:
  - [x] Read `flowId` from `?flow=`
  - [x] If missing → `throw redirect(initUrl("recovery"))`
  - [x] Call `frontend.getRecoveryFlow({ id: flowId, cookie: getCookie(request) })`
  - [x] Catch → `await handleFlowError(error, "recovery", request)`
  - [x] Return `{ flow }`
- [x] Component — state-driven:
  - [x] `flow.state === "choose_method"` → render heading "Recover your account" + neutral guidance text + `<FlowForm ui={flow.ui} />`
  - [x] `flow.state === "sent_email"` → render heading "Check your email" + neutral guidance text + `<FlowForm ui={flow.ui} />`
  - [x] `flow.state === "passed_challenge"` → render "Redirecting…" (Kratos transitions to settings)
  - [x] Default → render "Redirecting…"
  - [x] Render `flow.ui.messages` verbatim using `<FlowMessages />` regardless of state (neutral by design)

#### `app/routes/verification.tsx`

- [x] Loader:
  - [x] Read `flowId` from `?flow=`
  - [x] If missing → `throw redirect(initUrl("verification"))`
  - [x] Call `frontend.getVerificationFlow({ id: flowId, cookie: getCookie(request) })`
  - [x] Catch → `await handleFlowError(error, "verification", request)`
  - [x] Return `{ flow }`
- [x] Component — state-driven:
  - [x] `flow.state === "choose_method"` → render heading "Verify your email" + `<FlowForm ui={flow.ui} />`
  - [x] `flow.state === "sent_email"` → render heading "Enter verification code" + `<FlowForm ui={flow.ui} />`
  - [x] `flow.state === "passed_challenge"` → render heading "Email verified" + success message + `<a href="/settings">Go to settings</a>`
  - [x] Default → render fallback with link to `/`

---

### Phase 5 — Verification

- [x] Run `pnpm typecheck` (`react-router typegen && tsc`) — fix all TypeScript errors
- [x] Confirm every SSR Kratos call passes `cookie: getCookie(request)` — audit all 7 SDK call sites
- [x] Confirm no hardcoded field names exist anywhere in route or component files
- [x] Confirm every form in the app uses a plain HTML `<form>` (not React Router `<Form>`) with `action` and `method` taken from `flow.ui`
- [x] Confirm `routes.ts` paths match the URL conventions used by Kratos (`selfservice.flows.*.ui_url` config values: `/login`, `/registration`, `/settings`, `/recovery`, `/verification`, `/error`)
