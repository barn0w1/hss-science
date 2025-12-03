import { createRequestHandler } from "react-router";
import { api } from "../app/lib/api";

declare module "react-router" {
  export interface AppLoadContext {
    cloudflare: {
      env: Env;
      ctx: ExecutionContext;
    };
  }
}

const requestHandler = createRequestHandler(
  () => import("virtual:react-router/server-build"),
  import.meta.env.MODE
);

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // API routes are handled by Hono
    if (url.pathname.startsWith('/api')) {
      return api.fetch(request, env, ctx);
    }

    // All other routes are handled by React Router
    return requestHandler(request, {
      cloudflare: { env, ctx },
    });
  },
} satisfies ExportedHandler<Env>;
