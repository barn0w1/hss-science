import { redirect } from "react-router";
import type { Route } from "./+types/recovery";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";
import { FlowMessages } from "~/components/FlowMessages";

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const flowId = url.searchParams.get("flow");

  if (!flowId) {
    throw redirect(initUrl("recovery"));
  }

  try {
    const flow = await frontend.getRecoveryFlow({
      id: flowId,
      cookie: getCookie(request),
    });
    return { flow };
  } catch (error) {
    return await handleFlowError(error, "recovery", request);
  }
}

export default function Recovery({ loaderData }: Route.ComponentProps) {
  const { flow } = loaderData;

  return (
    <div>
      <FlowMessages messages={flow.ui.messages} />
      {flow.state === "choose_method" && (
        <>
          <h1>Recover your account</h1>
          <p>Enter your email address and we&apos;ll send a recovery code.</p>
          <FlowForm ui={flow.ui} />
        </>
      )}
      {flow.state === "sent_email" && (
        <>
          <h1>Check your email</h1>
          <p>A recovery code has been sent. Enter it below.</p>
          <FlowForm ui={flow.ui} />
        </>
      )}
      {(flow.state === "passed_challenge" ||
        (flow.state !== "choose_method" && flow.state !== "sent_email")) && (
        <p>Redirecting&hellip;</p>
      )}
    </div>
  );
}
