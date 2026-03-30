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

Kratos embeds the original `return_to` value in the flow object. Access it as `flow.return_to` (type: `string | undefined`) — verify against SDK types before using; cast if needed.

In login component: pass `flow.return_to` to `initUrl("registration", ...)` and `initUrl("recovery", ...)`.
In registration component: pass `flow.return_to` to `initUrl("login", ...)`.

---

## 4. File changes summary

| File | Change type |
|---|---|
| `web/apps/accounts/package.json` | Add `pino` + `@types/pino` |
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

- [ ] Run `pnpm dlx shadcn@latest init --preset base-nova --yes` from `web/apps/accounts/`
  - Verify `components.json` is created with `framework: react-router`, Tailwind v4, alias `~/`
  - Verify `app/lib/utils.ts` was created with `cn()` using `clsx` + `tailwind-merge`
  - Verify `app/app.css` now contains `:root { --background: ...; --foreground: ...; ... }` CSS variables
- [ ] Add shadcn components:
  - `pnpm dlx shadcn@latest add button input label card alert separator badge --yes`
  - After adding, read each generated file in `app/components/ui/` to understand the API
  - Verify `Alert` has `"default"` and `"destructive"` variants; note exact variant types
- [ ] Add `"success"` variant to `app/components/ui/alert.tsx`:
  - Extend `alertVariants` cva with a `success` variant using semantic or Tailwind classes (`border-green-600 text-green-700 [&>svg]:text-green-600` or equivalent)
- [ ] Run `pnpm typecheck` — fix any errors introduced by shadcn init

---

### Phase B — Logging

- [ ] Add `pino` to `web/apps/accounts/package.json` dependencies; add `@types/pino` to devDependencies
- [ ] Run `pnpm install --filter accounts` from `web/`
- [ ] Create `app/lib/logger.server.ts`:
  - Import `pino` (type: `import pino from "pino"`)
  - Create singleton: `const logger = pino({ level: process.env.LOG_LEVEL ?? "info", redact: ["cookie", "authorization", "*.cookie", "*.authorization"] })`
  - Export `logger`
  - Export `createRequestLogger(request: Request)` — returns `logger.child({ method: request.method, pathname: new URL(request.url).pathname, requestId: request.headers.get("x-request-id") ?? undefined })`
  - File must only be imported server-side (no `.client.ts` extension needed — loaders are SSR-only, but name the file `.server.ts` to enforce bundling boundary)

---

### Phase C — Component upgrades

#### `app/components/FlowMessages.tsx`

- [ ] Add `variant` prop: `"alert"` | `"field"` (default: `"alert"`)
- [ ] For `variant="alert"`:
  - Render each message as `<Alert variant={...}>` + `<AlertDescription>`
  - Map `message.type`: `"error"` → `"destructive"`, `"success"` → `"success"`, `"info"` → `"default"`
  - Wrap all alerts in a `<div className="flex flex-col gap-2">`
- [ ] For `variant="field"`:
  - Render each message as `<p className="text-sm ...">`
  - Map colors: `"error"` → `text-destructive`, `"success"` → `text-green-600`, `"info"` → `text-muted-foreground`
- [ ] Import `Alert`, `AlertDescription` from `~/components/ui/alert`
- [ ] Ensure no TypeScript errors (verify exact variant types from alert.tsx)

#### `app/components/FlowForm.tsx`

- [ ] Import `Button` from `~/components/ui/button`
- [ ] Import `Input` from `~/components/ui/input`
- [ ] Import `Label` from `~/components/ui/label`
- [ ] Import `Separator` from `~/components/ui/separator`
- [ ] Import `cn` from `~/lib/utils`
- [ ] Add `GROUP_LABELS` map at module level:
  ```ts
  const GROUP_LABELS: Record<string, string> = {
    password: "Password",
    profile: "Profile",
    oidc: "Social accounts",
    code: "Verification code",
    totp: "Authenticator app",
    webauthn: "Security key",
    passkey: "Passkey",
    lookup_secret: "Recovery codes",
  };
  ```
  Fallback for unknown groups: `group.charAt(0).toUpperCase() + group.slice(1)`
- [ ] Update `renderNode` for `UiNodeTypeEnum.Input`:
  - `type === "hidden"`: keep raw `<input>` (hidden, no wrapper, no label)
  - `type === "submit"` or `type === "button"`: render `<Button type={attrs.type} name={attrs.name} value={nodeValue(attrs.value) ?? ""} disabled={attrs.disabled}>{node.meta.label?.text ?? "Submit"}</Button>`
  - All other types: render:
    ```tsx
    <div key={key} className="flex flex-col gap-1.5">
      {/* Label — hidden only when node.meta.label is absent */}
      {node.meta.label?.text && (
        <Label htmlFor={attrs.name}>{node.meta.label.text}</Label>
      )}
      <Input
        id={attrs.name}
        name={attrs.name}
        type={attrs.type as string}
        required={attrs.required}
        disabled={attrs.disabled}
        autoComplete={attrs.autocomplete as string | undefined}
        aria-invalid={node.messages.length > 0 || undefined}
        {...(isSubmit ? { value: nodeValue(attrs.value) ?? "" } : { defaultValue: nodeValue(attrs.value) })}
      />
      <FlowMessages messages={node.messages} variant="field" />
    </div>
    ```
- [ ] Update group rendering: replace bare `<h2>` with a `<Separator>` + styled heading for non-default groups:
  ```tsx
  <section key={group} className="flex flex-col gap-4">
    <div className="flex items-center gap-3">
      <Separator className="flex-1" />
      <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
        {GROUP_LABELS[group] ?? (group.charAt(0).toUpperCase() + group.slice(1))}
      </span>
      <Separator className="flex-1" />
    </div>
    {nodes.map((node, i) => renderNode(node, i))}
  </section>
  ```
- [ ] Wrap the default group in `<div className="flex flex-col gap-4">` instead of bare `<div>`
- [ ] Wrap the outer form body in `<div className="flex flex-col gap-6">` for consistent spacing
- [ ] Run `pnpm typecheck` after changes; fix any errors

#### New: `app/components/AuthCard.tsx`

- [ ] Create with props:
  ```ts
  interface AuthCardProps {
    title: string;
    description?: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }
  ```
- [ ] Render:
  ```tsx
  <div className="min-h-screen flex items-center justify-center p-4 bg-background">
    <Card className="w-full max-w-md">
      <CardHeader className="text-center">
        <CardTitle className="text-2xl">{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
      {footer && (
        <CardFooter className="flex flex-col gap-2 text-center text-sm text-muted-foreground">
          {footer}
        </CardFooter>
      )}
    </Card>
  </div>
  ```
- [ ] Import `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter` from `~/components/ui/card`

---

### Phase D — Route page UI + meta() + ErrorBoundary

For each route, add:
1. `export function meta()` returning a page title
2. A per-route `ErrorBoundary` component
3. Updated component layout using `AuthCard` or page layout

#### `app/routes/login.tsx`

- [ ] Add `meta()` export: `[{ title: "Sign in — HSS Science" }]`
- [ ] Wrap component in `AuthCard`:
  - `title="Sign in"`
  - `footer=` with links: "Don't have an account? **Create account**" and "**Forgot password?**"
  - Pass `flow.return_to` to `initUrl("registration", flow.return_to ?? undefined)` and `initUrl("recovery", flow.return_to ?? undefined)`. Access `return_to` from `flow` — verify SDK type; use `(flow as { return_to?: string }).return_to` if the SDK types it as `object`
- [ ] Add `ErrorBoundary`:
  ```tsx
  export function ErrorBoundary() {
    return (
      <AuthCard title="Sign in">
        <Alert variant="destructive">
          <AlertDescription>
            Something went wrong. <a href="/login" className="underline">Try again</a>
          </AlertDescription>
        </Alert>
      </AuthCard>
    );
  }
  ```

#### `app/routes/registration.tsx`

- [ ] Add `meta()` export: `[{ title: "Create account — HSS Science" }]`
- [ ] Wrap component in `AuthCard`:
  - `title="Create account"`
  - `footer=` with link: "Already have an account? **Sign in**" (pass `flow.return_to`)
- [ ] Add `ErrorBoundary` (same pattern as login)

#### `app/routes/recovery.tsx`

- [ ] Add `meta()` export: `[{ title: "Recover account — HSS Science" }]`
- [ ] Wrap state-driven content in `AuthCard`:
  - `choose_method`: `title="Recover your account"`, `description="Enter your email and we'll send a recovery code."`
  - `sent_email`: `title="Check your email"`, `description="Enter the recovery code sent to your inbox."`
  - `passed_challenge` / default: render inside a plain centered div (no Card needed — transient state)
  - Move `<FlowMessages messages={flow.ui.messages} />` inside the AuthCard (above FlowForm)
- [ ] Add `ErrorBoundary`

#### `app/routes/verification.tsx`

- [ ] Add `meta()` export: `[{ title: "Verify email — HSS Science" }]`
- [ ] Wrap in `AuthCard`:
  - `choose_method`: `title="Verify your email"`, `description="Enter your email address to receive a verification code."`
  - `sent_email`: `title="Enter verification code"`, `description="Check your inbox and enter the code below."`
  - `passed_challenge`: `title="Email verified"` — render success content inside AuthCard content area, with link to settings
  - default: `title="Verification"` — generic fallback
- [ ] Add `ErrorBoundary`

#### `app/routes/settings.tsx`

- [ ] Add `meta()` export: `[{ title: "Account settings — HSS Science" }]`
- [ ] Update component layout (wider page layout, not AuthCard):
  ```tsx
  <div className="max-w-2xl mx-auto px-4 py-8 flex flex-col gap-8">
    <div className="flex items-start justify-between">
      <div>
        <h1 className="text-2xl font-semibold">Account settings</h1>
        {traits?.email && (
          <p className="text-sm text-muted-foreground mt-1">{traits.email}</p>
        )}
      </div>
      <Button variant="outline" asChild>
        <a href="/logout">Sign out</a>
      </Button>
    </div>
    {flow.state === "success" && (
      <Alert variant="success">
        <AlertDescription>Settings saved successfully.</AlertDescription>
      </Alert>
    )}
    <FlowForm ui={flow.ui} />
  </div>
  ```
- [ ] Add `ErrorBoundary`:
  ```tsx
  export function ErrorBoundary() {
    return (
      <div className="max-w-2xl mx-auto px-4 py-8">
        <Alert variant="destructive">
          <AlertDescription>
            Something went wrong. <a href="/settings" className="underline">Try again</a>
          </AlertDescription>
        </Alert>
      </div>
    );
  }
  ```

#### `app/routes/error.tsx`

- [ ] Add `meta()` export: `[{ title: "Error — HSS Science" }]`
- [ ] Wrap in `AuthCard`:
  - `flowError === null`: `title="Something went wrong"` — generic message in AuthCard content
  - `flowError` present: `title="Error${err.code ? ` ${err.code}` : ""}"` — show message + reason using Alert
  - Footer: "Go home" link
- [ ] No ErrorBoundary needed on this route (handled in loader)

#### `app/root.tsx` ErrorBoundary

- [ ] Import `Alert`, `AlertDescription` from `~/components/ui/alert`
- [ ] Import `Card`, `CardContent` from `~/components/ui/card`
- [ ] Update `ErrorBoundary` to use centered Card + Alert layout:
  ```tsx
  <div className="min-h-screen flex items-center justify-center p-4">
    <Card className="w-full max-w-md">
      <CardContent className="pt-6">
        <Alert variant="destructive">
          <AlertDescription>
            <strong>{status ? `Error ${status}` : "Error"}</strong>
            <br />{message}
          </AlertDescription>
        </Alert>
        <a href="/" className="block mt-4 text-sm text-center underline text-muted-foreground">
          Go home
        </a>
      </CardContent>
    </Card>
  </div>
  ```

---

### Phase E — Logging integration

- [ ] Update `app/lib/errors.ts`:
  - Import `logger` from `~/lib/logger.server`
  - At each classification branch, add a `logger.info` or `logger.warn` call:
    - `session_already_available` → `logger.info({ flowType }, "flow: session already available, redirecting to settings")`
    - `self_service_flow_expired` → `logger.info({ flowType }, "flow: expired, reinitializing")`
    - `session_inactive` → `logger.info({ flowType }, "flow: session inactive, redirecting to login")`
    - `session_refresh_required` → `logger.info({ flowType }, "flow: privilege refresh required")`
    - `browser_location_change_required` → `logger.info({ flowType, redirectBrowserTo }, "flow: browser location change required")`
    - Default 502 branch → `logger.warn({ flowType, status: error.response.status, errorId }, "flow: unhandled upstream error, returning 502")`
  - Non-ResponseError rethrow → `logger.error({ err: error, flowType }, "flow: unexpected non-ResponseError")`
- [ ] Update `app/lib/session.ts`:
  - Import `logger`
  - In `getSession`, before re-throwing a non-401 ResponseError: `logger.warn({ status: error.response.status }, "session check returned unexpected status")`
  - Before re-throwing non-ResponseError: `logger.error({ err: error }, "session check failed with unexpected error")`
- [ ] Update `app/entry.server.tsx`:
  - Import `logger` from `~/lib/logger.server`
  - Add `export function handleError(error: unknown, { request }: { request: Request }): void` (verify exact `handleError` signature from React Router v7 docs before implementing; use `ask_expert` if uncertain):
    ```ts
    export function handleError(error: unknown, { request }: { request: Request }): void {
      if (request.signal.aborted) return; // skip intentionally cancelled requests
      logger.error({ err: error, url: new URL(request.url).pathname }, "unhandled server error");
    }
    ```
  - Add security headers in `onShellReady` before `pipe(body)`:
    ```ts
    responseHeaders.set("X-Frame-Options", "DENY");
    responseHeaders.set("X-Content-Type-Options", "nosniff");
    responseHeaders.set("Referrer-Policy", "strict-origin-when-cross-origin");
    responseHeaders.set("Permissions-Policy", "geolocation=(), camera=(), microphone=()");
    ```

---

### Phase F — Typecheck and final verification

- [ ] Run `pnpm typecheck` from `web/apps/accounts/`; fix all TypeScript errors
- [ ] Verify `Alert` variant type includes `"success"` after modification
- [ ] Verify `meta()` return types match `Route.MetaArgs` for each route
- [ ] Verify `handleError` signature matches React Router v7 exported type
- [ ] Verify `flow.return_to` access is type-safe (no TypeScript errors)
- [ ] Confirm all new imports use `~/` alias prefix
- [ ] Confirm `logger.server.ts` is not imported in any `.client.tsx` or component file that could be bundled client-side
- [ ] Confirm no `any` or `unknown` type assertions have been added beyond what the existing codebase already uses

---

## 6. Notes and open questions

### shadcn/ui init output
After running `pnpm dlx shadcn@latest init`, the CLI modifies `app.css`. Read the resulting CSS carefully before Phase C — the `Alert` variant colors depend on the CSS variables that were installed.

### pino types
pino ships `@types/pino` separately for older versions. For pino v8+, types are bundled. Check the installed version after `pnpm install` and adjust devDependencies accordingly.

### `flow.return_to` SDK type
Before implementing Phase D return_to propagation, verify the field name and type in `@ory/kratos-client-fetch`:
- Check `LoginFlow` interface for a `return_to` or `returnTo` field
- Use `ask_expert` if uncertain

### React Router v7 `handleError` signature
The exact type is: `(error: unknown, { request, params, context }: DataStrategyFunctionArgs) => void | Promise<void>`. Only `request` is needed. Verify by checking the `@react-router/node` types after installation or via ask_expert.

### Alert success variant
shadcn's `Alert` cva uses `border` and `text-foreground` variants. The success variant should follow the same pattern using Tailwind utilities (`border-green-500/50 text-green-700 dark:text-green-400 [&>svg]:text-green-600`). After shadcn init, read `alert.tsx` to get the exact cva structure before adding the variant.

### Styling constraint: no raw color overrides
Per shadcn rules, use semantic color tokens (`text-destructive`, `text-muted-foreground`) not raw Tailwind palette values (`text-red-600`). The existing `FlowMessages.tsx` uses `text-red-600` — this will be replaced in Phase C.
