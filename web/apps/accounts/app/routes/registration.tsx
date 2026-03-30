import { redirect } from "react-router";
import type { Route } from "./+types/registration";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";
import { AuthCard } from "~/components/AuthCard";
import { Alert, AlertDescription } from "~/components/ui/alert";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Create account — HSS Science" }];
}

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
    <AuthCard
      title="Create account"
      footer={
        <span>
          Already have an account?{" "}
          <a href={initUrl("login", flow.return_to ?? undefined)} className="underline underline-offset-4 text-foreground hover:text-foreground/80">
            Sign in
          </a>
        </span>
      }
    >
      <FlowForm ui={flow.ui} />
    </AuthCard>
  );
}

export function ErrorBoundary() {
  return (
    <AuthCard title="Create account">
      <Alert variant="destructive">
        <AlertDescription>
          Something went wrong.{" "}
          <a href="/registration" className="underline">
            Try again
          </a>
        </AlertDescription>
      </Alert>
    </AuthCard>
  );
}
