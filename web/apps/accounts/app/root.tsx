import "./app.css";
import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";
import type { Route } from "./+types/root";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Card, CardContent } from "~/components/ui/card";

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = "An unexpected error occurred.";
  let status: number | undefined;

  if (isRouteErrorResponse(error)) {
    status = error.status;
    message = error.data ?? error.statusText;
  } else if (error instanceof Error) {
    message = error.message;
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-background">
      <Card className="w-full max-w-md">
        <CardContent className="pt-6">
          <Alert variant="destructive">
            <AlertDescription>
              <strong>{status ? `Error ${status}` : "Error"}</strong>
              <br />
              {message}
            </AlertDescription>
          </Alert>
          <a
            href="/"
            className="block mt-4 text-sm text-center underline underline-offset-4 text-muted-foreground hover:text-foreground"
          >
            Go home
          </a>
        </CardContent>
      </Card>
    </div>
  );
}
