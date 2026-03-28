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
1. Browser navigates to Kratos init: KRATOS_BROWSER_URL/self-service/<flow>/browser
   → Kratos creates flow, sets anti-CSRF cookie, 303s to our UI: /login?flow=<id>

2. Our loader fetches: GET KRATOS_PUBLIC_URL/self-service/<flow>/flows?id=<id>
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
| `KRATOS_PUBLIC_URL` | `http://kratos-public.identity.svc.cluster.local` | SSR → Kratos (internal) |
| `KRATOS_BROWSER_URL` | `https://accounts.hss-science.org` | Browser → Kratos (external) |

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
export const KRATOS_BROWSER_URL: string  // = process.env.KRATOS_BROWSER_URL ?? "https://accounts.hss-science.org"

// Extract Cookie header string from a Request
export function getCookie(request: Request): string | undefined

// Build the URL to initialize a browser flow
// e.g. initUrl("login") → "https://accounts.hss-science.org/self-service/login/browser"
// e.g. initUrl("login", "https://...") → appends ?return_to=<encoded>
export function initUrl(flow: string, returnTo?: string): string
```

**Implementation notes:**
- Instantiate `new FrontendApi(new Configuration({ basePath: KRATOS_PUBLIC_URL }))`
- `getCookie` reads `request.headers.get("cookie") ?? undefined`
- `initUrl` builds `${KRATOS_BROWSER_URL}/self-service/${flow}/browser` and appends `?return_to=...` only if `returnTo` is provided

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
| status 410 or `error.id === "self_service_flow_expired"` | `throw redirect(initUrl(flowType))` |
| status 401 / `session_inactive` | `throw redirect("/login")` |
| status 403 / `session_refresh_required` | `throw redirect(initUrl("login") + "?refresh=true")` |
| status 422 / `browser_location_change_required` | Read `redirect_browser_to` from body; `throw redirect(redirectBrowserTo)` |
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
   - 403 / `session_refresh_required` → `throw redirect(initUrl("login") + "?refresh=true&return_to=" + encodeURIComponent("/settings"))`
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

default (unexpected state):
  <p>Redirecting...</p>
  — Kratos handles the final redirect to /settings on success
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
| Flow expired | 410 | `self_service_flow_expired` | `redirect(initUrl(flowType))` |
| No active session | 401 | `session_inactive` | `redirect("/login")` |
| Privileged session required | 403 | `session_refresh_required` | `redirect(initUrl("login") + "?refresh=true")` |
| Browser location change | 422 | `browser_location_change_required` | `redirect(redirect_browser_to)` |
| Kratos system error | — | — | Kratos redirects browser to `/error?id=<id>` |
| Unexpected upstream error | 5xx | — | `throw data(...)` → route `ErrorBoundary` |
| Unexpected JS error | — | — | root `ErrorBoundary` |

All `ResponseError` parsing uses `error.response.clone().json()` wrapped in try/catch to guard against non-JSON bodies.

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

- TOTP / WebAuthn / Passkeys / OIDC — do not implement, but `FlowForm` must render `text`, `img`, `script` node types gracefully (no crashes) in case they appear
- AAL2 step-up flows
- i18n / message ID localization (render `message.text` as-is)
- Polished UI / design system
- Client-side validation
- Session token / native API flows
