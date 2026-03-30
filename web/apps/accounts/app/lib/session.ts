import { redirect } from "react-router";
import type { Session } from "@ory/kratos-client-fetch";
import { ResponseError } from "@ory/kratos-client-fetch";
import { frontend, getCookie } from "./kratos";
import { logger } from "./logger.server";

export async function getSession(request: Request): Promise<Session | null> {
  try {
    return await frontend.toSession({ cookie: getCookie(request) });
  } catch (error) {
    if (error instanceof ResponseError) {
      if (error.response.status === 401) {
        return null;
      }
      logger.warn({ status: error.response.status }, "session check returned unexpected status");
    } else {
      logger.error({ err: error }, "session check failed with unexpected error");
    }
    throw error;
  }
}

export async function requireSession(request: Request): Promise<Session> {
  const session = await getSession(request);
  if (session === null) {
    throw redirect("/login");
  }
  return session;
}
