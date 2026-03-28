import { redirect } from "react-router";
import type { Route } from "./+types/logout";
import { frontend, getCookie } from "~/lib/kratos";
import { ResponseError } from "@ory/kratos-client-fetch";

export async function loader({ request }: Route.LoaderArgs) {
  try {
    const logoutFlow = await frontend.createBrowserLogoutFlow({
      cookie: getCookie(request),
    });
    throw redirect(logoutFlow.logout_url);
  } catch (error) {
    if (error instanceof Response) {
      throw error;
    }
    if (error instanceof ResponseError && error.response.status === 401) {
      throw redirect("/login");
    }
    throw error;
  }
}

export default function Logout() {
  return null;
}
