import { redirect } from "react-router";
import type { Route } from "./+types/registration";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const flowId = url.searchParams.get("flow");

  if (!flowId) {
    throw redirect(initUrl("registration"));
  }

  try {
    const flow = await frontend.getRegistrationFlow({
      id: flowId,
      cookie: getCookie(request),
    });
    return { flow };
  } catch (error) {
    return await handleFlowError(error, "registration", request);
  }
}

export default function Registration({ loaderData }: Route.ComponentProps) {
  const { flow } = loaderData;

  return (
    <div>
      <h1>Create account</h1>
      <FlowForm ui={flow.ui} />
      <a href={initUrl("login")}>Already have an account? Sign in</a>
    </div>
  );
}
