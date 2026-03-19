import { setupWorker } from "msw/browser"
import { authHandlers } from "./handlers/auth"
import { spaceHandlers } from "./handlers/spaces"
import { nodeHandlers } from "./handlers/nodes"

export const worker = setupWorker(
  ...authHandlers,
  ...spaceHandlers,
  ...nodeHandlers
)
