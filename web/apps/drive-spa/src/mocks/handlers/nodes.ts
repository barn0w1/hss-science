import { http, HttpResponse } from "msw"
import { NODES } from "../fixtures"

export const nodeHandlers = [
  http.get("/api/spaces/:spaceId/nodes", ({ params, request }) => {
    const url = new URL(request.url)
    const parentId = url.searchParams.get("parentId") ?? null
    const items = NODES.filter(
      (n) => n.spaceId === params.spaceId && n.parentId === parentId
    )
    return HttpResponse.json({ items, total: items.length })
  }),

  http.get("/api/nodes/:nodeId", ({ params }) => {
    const node = NODES.find((n) => n.id === params.nodeId)
    if (!node) return HttpResponse.json({ error: "Not found" }, { status: 404 })
    return HttpResponse.json(node)
  }),
]
