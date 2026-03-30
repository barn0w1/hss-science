import { redirect } from "react-router";
import type { Route } from "./+types/settings";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { requireSession } from "~/lib/session";
import { handleFlowError } from "~/lib/errors";
import { ResponseError } from "@ory/kratos-client-fetch";
import { FlowForm } from "~/components/FlowForm";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Account settings — HSS Science" }];
}

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
    <div className="max-w-2xl mx-auto px-4 py-8 flex flex-col gap-8">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Account settings</h1>
          {traits?.email && (
            <p className="text-sm text-muted-foreground mt-1">{traits.email}</p>
          )}
        </div>
        <Button variant="outline" asChild>
          <a href="/logout">Sign out</a>
        </Button>
      </div>
      {flow.state === "success" && (
        <Alert variant="success">
          <AlertDescription>Settings saved successfully.</AlertDescription>
        </Alert>
      )}
      <FlowForm ui={flow.ui} />
    </div>
  );
}

export function ErrorBoundary() {
  return (
    <div className="max-w-2xl mx-auto px-4 py-8">
      <Alert variant="destructive">
        <AlertDescription>
          Something went wrong.{" "}
          <a href="/settings" className="underline">
            Try again
          </a>
        </AlertDescription>
      </Alert>
    </div>
  );
}
