import { http, HttpResponse } from "msw"

import type { components } from "@/api/generated"

type Profile = components["schemas"]["Profile"]
type Session = components["schemas"]["Session"]
type FederatedProvider = components["schemas"]["FederatedProvider"]
type UpdateProfileRequest = components["schemas"]["UpdateProfileRequest"]

const USER_ID = "01JX9K2N8BPQRS4TV5WXY6Z7A8"

let profile: Profile = {
  user_id: USER_ID,
  email: "alex.morgan@example.com",
  email_verified: true,
  name: "Alex Morgan",
  given_name: "Alex",
  family_name: "Morgan",
  picture: undefined,
  name_is_local: true,
  picture_is_local: false,
  created_at: "2024-03-15T09:22:00Z",
  updated_at: "2025-01-10T14:05:33Z",
}

let sessions: Session[] = [
  {
    session_id: "sess_01CURRENT",
    device_name: "Chrome on macOS",
    ip_address: "192.168.1.42",
    created_at: "2025-03-10T08:00:00Z",
    last_used_at: new Date(Date.now() - 3 * 60 * 1000).toISOString(),
    is_current: true,
  },
  {
    session_id: "sess_02IPHONE",
    device_name: "Safari on iPhone",
    ip_address: "10.0.0.5",
    created_at: "2025-02-28T19:30:00Z",
    last_used_at: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
    is_current: false,
  },
  {
    session_id: "sess_03TABLET",
    device_name: "Firefox on iPad",
    ip_address: "10.0.0.12",
    created_at: "2025-01-14T11:00:00Z",
    last_used_at: new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString(),
    is_current: false,
  },
]

let providers: FederatedProvider[] = [
  {
    identity_id: "idp_01GOOGLE",
    provider: "google",
    provider_email: "alex.morgan@gmail.com",
    last_login_at: new Date(Date.now() - 5 * 60 * 60 * 1000).toISOString(),
  },
  {
    identity_id: "idp_02GITHUB",
    provider: "github",
    provider_email: "alex-morgan",
    last_login_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
  },
]

export const handlers = [
  http.get("/api/v1/auth/me", () => {
    return HttpResponse.json({ logged_in: true, user_id: USER_ID })
  }),

  http.post("/api/v1/auth/logout", () => {
    return HttpResponse.json({ redirect_to: "/" })
  }),

  http.get("/api/v1/profile", () => {
    return HttpResponse.json(profile)
  }),

  http.patch("/api/v1/profile", async ({ request }) => {
    const body = (await request.json()) as UpdateProfileRequest
    profile = { ...profile, ...body, updated_at: new Date().toISOString() }
    return HttpResponse.json(profile)
  }),

  http.get("/api/v1/providers", () => {
    return HttpResponse.json(providers)
  }),

  http.delete("/api/v1/providers/:identityId", ({ params }) => {
    const identityId = params["identityId"] as string
    if (providers.length <= 1) {
      return HttpResponse.json(
        { error: "conflict", message: "You must keep at least one sign-in method" },
        { status: 409 },
      )
    }
    providers = providers.filter((p) => p.identity_id !== identityId)
    return new HttpResponse(null, { status: 204 })
  }),

  http.get("/api/v1/sessions", () => {
    return HttpResponse.json(sessions)
  }),

  http.delete("/api/v1/sessions", () => {
    sessions = sessions.filter((s) => s.is_current)
    return new HttpResponse(null, { status: 204 })
  }),

  http.delete("/api/v1/sessions/:sessionId", ({ params }) => {
    const sessionId = params["sessionId"] as string
    const exists = sessions.some((s) => s.session_id === sessionId)
    if (!exists) {
      return HttpResponse.json(
        { error: "not_found", message: "Session not found" },
        { status: 404 },
      )
    }
    sessions = sessions.filter((s) => s.session_id !== sessionId)
    return new HttpResponse(null, { status: 204 })
  }),
]
