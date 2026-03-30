import { data, redirect } from "react-router";
import {
  ResponseError,
  ErrorBrowserLocationChangeRequiredFromJSON,
} from "@ory/kratos-client-fetch";
import { initUrl } from "./kratos";
import { logger } from "./logger.server";

export async function handleFlowError(
  error: unknown,
  flowType: string,
  request: Request,
): Promise<never> {
  if (!(error instanceof ResponseError)) {
    logger.error({ err: error, flowType }, "flow: unexpected non-ResponseError");
    throw error;
  }

  const body = await error.response
    .clone()
    .json()
    .catch(() => undefined) as Record<string, unknown> | undefined;

  const errorId = (body?.error as Record<string, unknown> | undefined)
    ?.id as string | undefined;

  if (errorId === "session_already_available") {
    logger.info({ flowType }, "flow: session already available, redirecting to settings");
    throw redirect("/settings");
  }

  if (
    error.response.status === 410 ||
    errorId === "self_service_flow_expired"
  ) {
    logger.info({ flowType }, "flow: expired, reinitializing");
    throw redirect(initUrl(flowType));
  }

  if (error.response.status === 401 || errorId === "session_inactive") {
    logger.info({ flowType }, "flow: session inactive, redirecting to login");
    throw redirect("/login");
  }

  if (
    error.response.status === 403 ||
    errorId === "session_refresh_required"
  ) {
    logger.info({ flowType }, "flow: privilege refresh required");
    throw redirect(
      initUrl("login") +
        "?refresh=true&return_to=" +
        encodeURIComponent(request.url),
    );
  }

  if (
    error.response.status === 422 ||
    errorId === "browser_location_change_required"
  ) {
    let redirectBrowserTo: string | undefined;
    if (body !== undefined) {
      try {
        const parsed = ErrorBrowserLocationChangeRequiredFromJSON(body);
        redirectBrowserTo = parsed.redirect_browser_to;
      } catch {
        // parsing failed — fall through to 502
      }
    }
    if (redirectBrowserTo) {
      logger.info({ flowType, redirectBrowserTo }, "flow: browser location change required");
      throw redirect(redirectBrowserTo);
    }
    logger.warn({ flowType, status: error.response.status, errorId }, "flow: browser_location_change_required without redirect_browser_to");
    throw data("Upstream error", { status: 502 });
  }

  logger.warn({ flowType, status: error.response.status, errorId }, "flow: unhandled upstream error, returning 502");
  throw data("Upstream error", { status: 502 });
}
