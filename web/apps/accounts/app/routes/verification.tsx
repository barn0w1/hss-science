import { redirect } from "react-router";
import type { Route } from "./+types/verification";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const flowId = url.searchParams.get("flow");

  if (!flowId) {
    throw redirect(initUrl("verification"));
  }

  try {
    const flow = await frontend.getVerificationFlow({
      id: flowId,
      cookie: getCookie(request),
    });
    return { flow };
  } catch (error) {
    return await handleFlowError(error, "verification", request);
  }
}

export default function Verification({ loaderData }: Route.ComponentProps) {
  const { flow } = loaderData;

  if (flow.state === "choose_method") {
    return (
      <div>
        <h1>Verify your email</h1>
        <FlowForm ui={flow.ui} />
      </div>
    );
  }

  if (flow.state === "sent_email") {
    return (
      <div>
        <h1>Enter verification code</h1>
        <FlowForm ui={flow.ui} />
      </div>
    );
  }

  if (flow.state === "passed_challenge") {
    return (
      <div>
        <h1>Email verified</h1>
        <p>Your email address has been successfully verified.</p>
        <a href="/settings">Go to settings</a>
      </div>
    );
  }

  return (
    <div>
      <p>Something went wrong.</p>
      <a href="/">Go home</a>
    </div>
  );
}
