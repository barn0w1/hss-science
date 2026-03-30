import { redirect } from "react-router";
import type { Route } from "./+types/verification";
import { frontend, getCookie, initUrl } from "~/lib/kratos";
import { handleFlowError } from "~/lib/errors";
import { FlowForm } from "~/components/FlowForm";
import { AuthCard } from "~/components/AuthCard";
import { Alert, AlertDescription } from "~/components/ui/alert";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Verify email — HSS Science" }];
}

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
      <AuthCard
        title="Verify your email"
        description="Enter your email address to receive a verification code."
      >
        <FlowForm ui={flow.ui} />
      </AuthCard>
    );
  }

  if (flow.state === "sent_email") {
    return (
      <AuthCard
        title="Enter verification code"
        description="Check your inbox and enter the code below."
      >
        <FlowForm ui={flow.ui} />
      </AuthCard>
    );
  }

  if (flow.state === "passed_challenge") {
    return (
      <AuthCard
        title="Email verified"
        footer={
          <a href="/settings" className="underline underline-offset-4 hover:text-foreground">
            Go to settings
          </a>
        }
      >
        <p className="text-sm text-center text-muted-foreground">
          Your email address has been successfully verified.
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      title="Verification"
      footer={
        <a href="/" className="underline underline-offset-4 hover:text-foreground">
          Go home
        </a>
      }
    >
      <p className="text-sm text-center text-muted-foreground">Something went wrong.</p>
    </AuthCard>
  );
}

export function ErrorBoundary() {
  return (
    <AuthCard title="Verify email">
      <Alert variant="destructive">
        <AlertDescription>
          Something went wrong.{" "}
          <a href="/verification" className="underline">
            Try again
          </a>
        </AlertDescription>
      </Alert>
    </AuthCard>
  );
}
