import type { Route } from "./+types/error";
import { frontend } from "~/lib/kratos";

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
      <div>
        <h1>Something went wrong</h1>
        <p>An unexpected error occurred.</p>
        <a href="/">Go home</a>
      </div>
    );
  }

  const err = flowError.error as {
    code?: number;
    message?: string;
    reason?: string;
  };

  return (
    <div>
      <h1>Error{err.code ? ` ${err.code}` : ""}</h1>
      {err.message && <p>{err.message}</p>}
      {err.reason && <p>{err.reason}</p>}
      <a href="/">Go home</a>
    </div>
  );
}
