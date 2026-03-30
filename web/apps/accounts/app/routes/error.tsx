import type { Route } from "./+types/error";
import { frontend } from "~/lib/kratos";
import { AuthCard } from "~/components/AuthCard";
import { Alert, AlertDescription } from "~/components/ui/alert";

export function meta(): Route.MetaDescriptors {
  return [{ title: "Error — HSS Science" }];
}

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const errorId = url.searchParams.get("id");

  if (!errorId) {
    return { flowError: null };
  }

  try {
    const flowError = await frontend.getFlowError({ id: errorId });
    return { flowError };
  } catch {
    return { flowError: null };
  }
}

export default function ErrorPage({ loaderData }: Route.ComponentProps) {
  const { flowError } = loaderData;

  if (!flowError) {
    return (
      <AuthCard
        title="Something went wrong"
        footer={
          <a href="/" className="underline underline-offset-4 hover:text-foreground">
            Go home
          </a>
        }
      >
        <Alert variant="destructive">
          <AlertDescription>An unexpected error occurred.</AlertDescription>
        </Alert>
      </AuthCard>
    );
  }

  const err = flowError.error as {
    code?: number;
    message?: string;
    reason?: string;
  };

  return (
    <AuthCard
      title={`Error${err.code ? ` ${err.code}` : ""}`}
      footer={
        <a href="/" className="underline underline-offset-4 hover:text-foreground">
          Go home
        </a>
      }
    >
      <Alert variant="destructive">
        <AlertDescription>
          {err.message && <p>{err.message}</p>}
          {err.reason && <p className="mt-1 text-xs opacity-80">{err.reason}</p>}
        </AlertDescription>
      </Alert>
    </AuthCard>
  );
}
