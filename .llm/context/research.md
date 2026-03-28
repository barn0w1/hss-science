# Ory Kratos Self-Service UI Integration — Research

## Deployment context

| Item | Value |
|---|---|
| Platform | Kubernetes |
| Flow type | Browser flow (server-side application / SSR) |
| Kratos public URL (SSR → Kratos, internal) | `http://kratos-public.identity.svc.cluster.local` |
| Kratos browser URL (browser-facing, external) | `https://accounts.hss-science.org` |

The SSR application calls Kratos at the internal cluster URL. The browser never calls Kratos directly; all self-service endpoints are proxied through or redirected by the SSR application.

---

## 1. Core concepts

### What Kratos does

Kratos is a **headless identity and authentication backend**. It owns:

- Identity storage (traits, credentials, verification/recovery addresses)
- Self-service flow state machines (login, registration, settings, recovery, verification, logout)
- Security controls: CSRF protection, session cookie/token issuance, redirect-safety checks, privileged-session enforcement
- Flow payload generation: `ui.action`, `ui.method`, `ui.nodes`, `ui.messages`
- Validation against identity schema and enabled method requirements
- Hooks before/after flow stages
- Session management (`/sessions/whoami`)

### What Kratos does not do

- Render HTML for any end-user page
- Supply an end-user UI or templating engine
- Act as an OAuth2/OIDC authorization server (that is Ory Hydra)
- Provide WAF, bot mitigation, or rate-limiting (operator responsibility in self-hosted deployments)
- Reset second-factor configuration during account recovery

### The role of the UI application

The UI application is the **flow renderer and orchestrator**. It:

- Hosts all user-facing routes (`/login`, `/registration`, `/settings`, `/recovery`, `/verification`, `/error`)
- Receives `?flow=<id>` query parameters when Kratos redirects the browser to the UI
- Fetches the flow object from Kratos using that ID
- Renders form fields and messages **dynamically from `flow.ui`** — never hardcoded
- Submits form data to exactly `flow.ui.action` with method `flow.ui.method`
- Interprets Kratos responses (validation errors, redirects, 422s, success) and drives the next UX state

Mental model: **Kratos owns state machine, policy, validation, and session outcomes. The UI owns rendering, event wiring, navigation, and user experience.**

---

## 2. Browser flow mechanics

### Two browser sub-modes

| Mode | Init endpoint | `Accept` header | Kratos response | Cookie/CSRF |
|---|---|---|---|---|
| SSR / redirect | `/self-service/<flow>/browser` | `text/html` | `303` redirect to UI route with `?flow=<id>` | Browser cookies set; CSRF enforced |
| SPA / AJAX | `/self-service/<flow>/browser` | `application/json` | `200` JSON with flow object | Browser cookies still required; `credentials: include` |
| Native / API | `/self-service/<flow>/api` | `application/json` | `200` JSON with flow object | No browser cookies; session token returned on success |

This project uses **SSR browser flows**.

### Full SSR lifecycle

```
1. Browser → GET /self-service/login/browser  (Accept: text/html)
             → Kratos creates flow, sets anti-CSRF cookie
             → 303 to https://accounts.hss-science.org/login?flow=<id>

2. SSR app   → GET http://kratos-public.../self-service/login/flows?id=<id>
             ← 200 JSON: flow object with ui.action, ui.method, ui.nodes, ui.messages

3. SSR app renders HTML form from flow.ui (see §4)

4. Browser submits HTML form (application/x-www-form-urlencoded) to ui.action
   (ui.action points to Kratos browser URL, e.g. https://accounts.hss-science.org/self-service/login?flow=<id>)
   Browser cookies accompany the request automatically

5a. Validation error → Kratos returns updated flow with messages
    → SSR re-fetches and re-renders with error messages

5b. Success → Kratos issues session cookie + 303 to configured return URL
```

### Cookies and CSRF

- Kratos sets an **anti-CSRF cookie** when a browser flow is initialized.
- The flow payload includes a hidden `csrf_token` input node (in `group: default`).
- Kratos validates the cookie ↔ token pair (synchronizer token pattern) on every `POST`.
- **The SSR app must not strip or ignore cookies** passed from the browser on fetch calls to Kratos.
- For the SSR fetch to `/self-service/*/flows?id=...`, the `Cookie` header from the incoming browser request must be forwarded to Kratos.

### Domain constraints

- The browser-facing Kratos URL (`accounts.hss-science.org`) and the UI application must share the same domain (or a compatible subdomain) for cookies to work correctly across redirects.
- In this deployment both the UI and Kratos public API share `accounts.hss-science.org`, so the cookie domain constraint is satisfied.

### `return_to`

- A `return_to` query parameter can be passed when initializing flows to control the post-success redirect destination.
- Allowed values must be configured in `selfservice.allowed_return_urls` to prevent open redirects.
- `return_to` does **not** automatically persist across flow switches (e.g., login → registration) — the UI must carry it forward manually.
- Exception: recovery → settings preserves `return_to` automatically.

---

## 3. Self-service flows

### 3.1 Login

**Purpose:** Authenticate a user and issue a session (cookie for browser flows).

**Init (SSR):** `GET /self-service/login/browser` (`Accept: text/html`)

**Special parameters at init:**
- `?refresh=true` — force re-authentication even with an active session (updates `authenticated_at`)
- `?aal=aal2` — require second-factor step-up
- `?return_to=<url>` — post-success redirect

**Flow states:** initialized → UI renders → submission → (validation error loop) → success

**Active method node groups:** `password`, `oidc`, `code`, `passkey`, `webauthn` (depends on config)

**UI must handle:**
- Multiple method groups in one flow (render separately, submit one at a time)
- Social/OIDC submit buttons that must use native browser form POST (not AJAX)
- Code-based passwordless two-step UX when `code` method is enabled
- AAL2 step-up routing when `session_aal2_required` is returned from a protected resource

**Success:** `303` to configured default URL or `return_to`; browser receives session cookie.

---

### 3.2 Registration

**Purpose:** Create a new identity.

**Init (SSR):** `GET /self-service/registration/browser` (`Accept: text/html`)

**Flow states:** initialized → UI renders → submission → (validation loop) → success

**Active method node groups:** `password`, `oidc`, `profile` (trait fields), `code`

**Key behavior:**
- Form fields are generated from the **active identity schema** — do not assume email + password only.
- By default, successful registration does **not** issue a session; a `session` after-hook must be configured.
- OIDC registration may loop back to the UI if the provider omits required traits.

**UI must handle:**
- Dynamic trait fields driven by schema
- Post-registration branch: with session hook → treat as authenticated; without → route to login
- Potential follow-up verification requirement

---

### 3.3 Logout

**Purpose:** Terminate the current session.

**Pattern (SSR/browser):**
1. Fetch logout token: `GET /self-service/logout/browser` (with session cookie) → returns `{ logout_url, logout_token }`
2. Navigate browser to `logout_url` (or render a form that posts to it)

**`return_to`:** Can be appended to `logout_url` to control post-logout redirect.

**UI must handle:**
- Explicit user action (not silent logout)
- Clearing local auth state after the redirect completes

---

### 3.4 Settings

**Purpose:** Allow an authenticated user to update profile traits and credentials.

**Init (SSR):** `GET /self-service/settings/browser` (requires active session cookie)

**Flow states:** `show_form` → (submission) → `success` or back to `show_form` with errors

**Active method node groups:** `profile`, `password`, `oidc`, `totp`, `webauthn`, `passkey`, `lookup_secret`

**Key behavior — privileged sessions:**
- Certain changes (password, key traits, OIDC link/unlink, MFA factors) require a **privileged session**: `authenticated_at` must be within `privileged_session_max_age`.
- In strict MFA (`required_aal: highest_available`), settings may require AAL2 step-up.
- Browser: Kratos may redirect to a re-auth prompt automatically.
- API: Kratos returns `403`; app must re-run the login flow and retry.

**UI must handle:**
- Session precondition before initializing (redirect to login if no session)
- Distinct method forms per group (profile, password, OIDC, etc.)
- `state: show_form` vs `state: success` — render success message without leaving page (unless redirected)
- Privileged session / re-auth loop
- Link/unlink OIDC provider submit buttons

---

### 3.5 Account Recovery

**Purpose:** Regain account access (forgot password, lost 2FA).

**Init (SSR):** `GET /self-service/recovery/browser`

**Configured method:** `code` (recommended) or `link` (legacy, discouraged)

**Flow states (code method):**
1. `choose_method` — user enters recovery address
2. `sent_email` — code delivered; user enters code + resend controls available
3. Success → privileged session issued → `303` to settings for password reset

**Key behavior:**
- Recovery does **not** reset second-factor configuration.
- Successful recovery grants a privileged session and transitions user to settings.
- `return_to` persists from recovery into settings (special case).
- Optional: revoke all active sessions via `revoke_active_sessions` hook.
- Non-enumerating messaging: Kratos can notify unknown recipients without revealing account existence.

**UI must handle:**
- Two-step interaction: address entry → code entry with resend
- Neutral messaging (do not reveal account existence)
- Hand-off into settings password update after challenge success
- Flow expiry restart

---

### 3.6 Verification

**Purpose:** Verify ownership of an email or phone number tied to an identity trait.

**Init (SSR):** `GET /self-service/verification/browser`

**Configured method:** `code` (recommended) or `link` (legacy)

**Flow states (code method):**
1. `choose_method` — user enters address to verify
2. `sent_email` — code delivered; user enters code
3. `passed_challenge` — browser redirect completes back to verification UI with this state (unless overridden)

**Key behavior:**
- Traits must be marked verifiable in schema (`ory.sh/kratos.verification.via`).
- Can be triggered automatically after registration via after-registration hooks.
- `require_verified_address` login hook can enforce that only verified accounts may log in.
- Verified status can be carried from OIDC provider claims.

**UI must handle:**
- Two-step code UX: collect address → collect code → allow resend
- Rendering `state: passed_challenge` as a success screen
- Expired code/link → new challenge issuance
- Neutral messaging for unknown recipient scenarios

---

## 4. UI integration responsibilities

### 4.1 Rendering `ui.nodes`

Nodes are the **server-driven form schema**. The UI must never hardcode field names.

| Node `type` | Rendering requirement |
|---|---|
| `input` | Render as `<input>` with all attributes from node (`name`, `type`, `value`, `required`, `disabled`, etc.) |
| `text` | Display text content — used for TOTP secrets, lookup/recovery code lists; add copy/download affordances |
| `img` | Render image source (often a data URI) — used for TOTP QR code setup |
| `script` | Load script with exact attributes (`src`, `integrity`, `crossorigin`, `async`) — required for WebAuthn/Passkey |

Node attributes to respect: `name`, `type`, `value`, `required`, `disabled`, `label`, `node_type`, `autocomplete`, `onclick`.

**Grouping:** `node.group` identifies the method (`default`, `password`, `oidc`, `profile`, `code`, `webauthn`, `passkey`, `totp`, `lookup_secret`). Render groups as separate form sections or visually distinct areas, but all submit to the same `flow.ui.action`.

**Method dispatch:** Multiple submit nodes may share the same `name` (e.g., `method`) with different values. The UI must ensure only the **intended method value is submitted exactly once**. Wrong method dispatch will execute the wrong flow branch.

### 4.2 Rendering messages

Messages exist at two levels and must both be rendered:

- **Flow-level:** `flow.ui.messages[]` — root messages (e.g., "Invalid credentials", "An account with this email already exists")
- **Node-level:** `node.messages[]` — field-specific validation errors

Message IDs are stable and suitable for i18n. Always render the server-provided `text` as a fallback.

### 4.3 CSRF handling (browser SSR)

- **Forward browser cookies to Kratos** on every SSR-side fetch (flow init and flow fetch).
- Include the hidden `csrf_token` node in every form submission.
- Do not strip or rewrite the `Set-Cookie` headers Kratos sends back through the SSR proxy.
- On CSRF failure: do not retry blindly — re-initialize the flow to get fresh nodes and a fresh cookie.

### 4.4 Flow expiration handling

Flows have an `expires_at` timestamp. When a flow expires or an invalid flow ID is used:

1. Detect the error (status `410`, error ID `self_service_flow_expired`, or similar).
2. Initialize a new flow.
3. Re-render with the new flow ID.
4. Do not replay sensitive payloads.

### 4.5 Error flow — `/self-service/errors`

When Kratos encounters a user-facing system error in a browser flow:
- It redirects to the configured error UI URL with `?id=<error-id>`.
- The error page must call `GET /self-service/errors?id=<error-id>` and render the returned payload (`code`, `message`, `reason`, optional `debug`).
- Provide a safe fallback UX if the fetch fails.

The error UI URL is configured in Kratos at `selfservice.flows.error.ui_url`.

### 4.6 Session checking — `/sessions/whoami`

Use `GET /sessions/whoami` (forwarding the browser's session cookie) as the session truth source:

- Route guards: redirect unauthenticated users to login
- Auth state bootstrap on SSR page load
- Post-login verification

Key fields to read:

| Field | Use |
|---|---|
| `active` | Session validity |
| `expires_at` | Session expiry |
| `authenticated_at` | For privileged-session age checks |
| `authenticator_assurance_level` | `aal1` or `aal2` |
| `authentication_methods` | Which factors were used |
| `identity` | User identity data for rendering |

### 4.7 Handling `422 browser_location_change_required`

In SPA/native integration, Kratos may return `422` with `error.id = browser_location_change_required` and a `redirect_browser_to` URL. This is **not** a fatal error.

In this SSR project this scenario is less common, but may arise with OIDC or passkey methods. When encountered:
1. Read `redirect_browser_to` from the response body.
2. Issue a server-side redirect (`303`) to that URL, or return it to the browser for navigation.
3. Continue the flow from the new URL (which will include a `?flow=<id>` parameter).

### 4.8 Privileged sessions and AAL step-up

**Privileged session:** For sensitive settings actions, `authenticated_at` must be within `privileged_session_max_age`.
- Browser: Kratos may redirect to a re-auth screen automatically.
- Detect and surface as a security prompt, not a generic failure.
- After re-auth, return the user to the interrupted settings action.

**AAL step-up:** For operations requiring MFA:
- Initiate login with `?aal=aal2` to require second factor.
- Use `?refresh=true&aal=aal2` to force re-check even if already AAL2.
- Handle `session_aal2_required` errors from protected resources by routing to the step-up login flow.

### 4.9 URL and endpoint conventions

| Action | Endpoint | Caller |
|---|---|---|
| Init browser flow | `GET /self-service/<flow>/browser` | SSR app (forwards browser cookies) |
| Fetch flow data | `GET /self-service/<flow>/flows?id=<id>` | SSR app (forwards browser cookies) |
| Submit form | `POST <flow.ui.action>` | Browser (native HTML form POST) |
| Fetch error details | `GET /self-service/errors?id=<id>` | SSR app |
| Session check | `GET /sessions/whoami` | SSR app (forwards browser cookies) |
| Fetch logout token | `GET /self-service/logout/browser` | SSR app (forwards browser cookies) |

In this deployment, SSR-to-Kratos calls use `http://kratos-public.identity.svc.cluster.local`. `flow.ui.action` values returned by Kratos will contain the **external** Kratos browser URL (`https://accounts.hss-science.org/self-service/...`) — these are the URLs the **browser** submits forms to directly.

---

## 5. Summary of non-negotiable UI requirements

1. **Dynamic rendering** — render all forms from `flow.ui.nodes`; never hardcode field names or assume a fixed set of fields.
2. **All node types** — support `input`, `text`, `img`, and `script` nodes.
3. **All message levels** — render both `flow.ui.messages` and per-node `node.messages`.
4. **CSRF + cookies** — forward browser cookies on SSR fetches; include `csrf_token` hidden node in all form submissions.
5. **Flow expiration** — detect expired/invalid flow IDs, re-initialize, and re-render.
6. **Error flow page** — implement a dedicated error route that fetches and renders `/self-service/errors?id=<id>`.
7. **Session truth** — use `/sessions/whoami` for route guards and auth state.
8. **`422` handling** — detect `browser_location_change_required` and redirect to `redirect_browser_to`.
9. **`return_to`** — propagate `return_to` when chaining flows; use `selfservice.allowed_return_urls` to configure allowed targets.
10. **Privileged session / AAL** — detect and handle re-auth and step-up requirements gracefully.
11. **Method dispatch** — submit exactly the intended method value; avoid double-submitting method selectors.
12. **Non-enumeration** — use neutral messaging for recovery and verification flows that does not reveal account existence.
