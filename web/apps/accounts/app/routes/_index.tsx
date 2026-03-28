import { redirect } from "react-router";
import type { Route } from "./+types/_index";
import { getSession } from "~/lib/session";
import { initUrl } from "~/lib/kratos";

export async function loader({ request }: Route.LoaderArgs) {
  const session = await getSession(request);
  if (session !== null) {
    throw redirect("/settings");
  }
  throw redirect(initUrl("login"));
}

export default function Index() {
  return null;
}
