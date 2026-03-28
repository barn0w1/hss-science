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
    <div>
      <h1>{status ? `Error ${status}` : "Error"}</h1>
      <p>{message}</p>
      <a href="/">Go home</a>
    </div>
  );
}
