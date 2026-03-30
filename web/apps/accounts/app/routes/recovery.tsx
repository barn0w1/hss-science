import { redirect } from "react-router";
import type { Route } from "./+types/recovery";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";
import { FlowMessages } from "~/components/FlowMessages";
import { AuthCard } from "~/components/AuthCard";
import { Alert, AlertDescription } from "~/components/ui/alert";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Recover account — HSS Science" }];
}

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

  if (flow.state === "choose_method") {
    return (
      <AuthCard
        title="Recover your account"
        description="Enter your email address and we'll send a recovery code."
      >
        <FlowMessages messages={flow.ui.messages} />
        <FlowForm ui={flow.ui} />
      </AuthCard>
    );
  }

  if (flow.state === "sent_email") {
    return (
      <AuthCard
        title="Check your email"
        description="Enter the recovery code sent to your inbox."
      >
        <FlowMessages messages={flow.ui.messages} />
        <FlowForm ui={flow.ui} />
      </AuthCard>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-background">
      <p className="text-muted-foreground">Redirecting&hellip;</p>
    </div>
  );
}

export function ErrorBoundary() {
  return (
    <AuthCard title="Recover your account">
      <Alert variant="destructive">
        <AlertDescription>
          Something went wrong.{" "}
          <a href="/recovery" className="underline">
            Try again
          </a>
        </AlertDescription>
      </Alert>
    </AuthCard>
  );
}
