import { redirect } from "react-router";
import type { Route } from "./+types/settings";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { requireSession } from "~/lib/session";
import { handleFlowError } from "~/lib/errors";
import { ResponseError } from "@ory/kratos-client-fetch";
import { FlowForm } from "~/components/FlowForm";

export async function loader({ request }: Route.LoaderArgs) {
  const session = await requireSession(request);

  const url = new URL(request.url);
  const flowId = url.searchParams.get("flow");

  if (!flowId) {
    throw redirect(initUrl("settings"));
  }

  try {
    const flow = await frontend.getSettingsFlow({
      id: flowId,
      cookie: getCookie(request),
    });
    return { flow, session };
  } catch (error) {
    if (
      error instanceof ResponseError &&
      error.response.status === 403
    ) {
      throw redirect(
        initUrl("login") +
          "?refresh=true&return_to=" +
          encodeURIComponent(request.url),
      );
    }
    return await handleFlowError(error, "settings", request);
  }
}

export default function Settings({ loaderData }: Route.ComponentProps) {
  const { flow, session } = loaderData;
  const traits = session.identity?.traits as { email?: string } | undefined;

  return (
    <div>
      <h1>Account settings</h1>
      {traits?.email && <p>{traits.email}</p>}
      {flow.state === "success" && <p>Settings saved.</p>}
      <FlowForm ui={flow.ui} />
      <a href="/logout">Sign out</a>
    </div>
  );
}
