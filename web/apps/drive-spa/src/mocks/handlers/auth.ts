import { http, HttpResponse } from "msw"
import { ALICE } from "../fixtures"

export const authHandlers = [
  http.get("/api/me", () => HttpResponse.json(ALICE)),
]
