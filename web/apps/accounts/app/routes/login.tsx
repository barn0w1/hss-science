import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const flowId = url.searchParams.get("flow");

  if (!flowId) {
    throw redirect(initUrl("login"));
  }

  try {
    const flow = await frontend.getLoginFlow({
      id: flowId,
      cookie: getCookie(request),
    });
    return { flow };
  } catch (error) {
    return await handleFlowError(error, "login", request);
  }
}

export default function Login({ loaderData }: Route.ComponentProps) {
  const { flow } = loaderData;

  return (
    <div>
      <h1>Sign in</h1>
      <FlowForm ui={flow.ui} />
      <a href={initUrl("registration")}>Create account</a>
      <a href={initUrl("recovery")}>Forgot password?</a>
    </div>
  );
}
