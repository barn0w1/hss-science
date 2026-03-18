import { http, HttpResponse } from "msw"
import { PERSONAL_SPACE, TEAM_SPACE } from "../fixtures"

const ALL_SPACES = [PERSONAL_SPACE, TEAM_SPACE]

export const spaceHandlers = [
  http.get("/api/spaces", () => HttpResponse.json({ items: ALL_SPACES })),
  http.get("/api/spaces/:spaceId", ({ params }) => {
    const space = ALL_SPACES.find((s) => s.id === params.spaceId)
    if (!space)
      return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(space)
  }),
]
