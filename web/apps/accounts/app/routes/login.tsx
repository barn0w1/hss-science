import { redirect } from "react-router";
import type { Route } from "./+types/login";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";
import { AuthCard } from "~/components/AuthCard";
import { Alert, AlertDescription } from "~/components/ui/alert";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Sign in — HSS Science" }];
}

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
    <AuthCard
      title="Sign in"
      footer={
        <>
          <span>
            Don&apos;t have an account?{" "}
            <a href={initUrl("registration", flow.return_to ?? undefined)} className="underline underline-offset-4 text-foreground hover:text-foreground/80">
              Create account
            </a>
          </span>
          <a href={initUrl("recovery", flow.return_to ?? undefined)} className="underline underline-offset-4 hover:text-foreground">
            Forgot password?
          </a>
        </>
      }
    >
      <FlowForm ui={flow.ui} />
    </AuthCard>
  );
}

export function ErrorBoundary() {
  return (
    <AuthCard title="Sign in">
      <Alert variant="destructive">
        <AlertDescription>
          Something went wrong.{" "}
          <a href="/login" className="underline">
            Try again
          </a>
        </AlertDescription>
      </Alert>
    </AuthCard>
  );
}
