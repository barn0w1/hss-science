import createClient from "openapi-fetch"

import type { paths } from "./generated"

export const api = createClient<paths>({
  baseUrl: "/",
  headers: { "X-Requested-With": "XMLHttpRequest" },
})
