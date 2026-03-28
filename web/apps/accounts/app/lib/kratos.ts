import { Configuration, FrontendApi } from "@ory/kratos-client-fetch";

const KRATOS_PUBLIC_URL =
  process.env.KRATOS_PUBLIC_URL ??
  "http://kratos-public.identity.svc.cluster.local";

export const KRATOS_BROWSER_URL =
  process.env.KRATOS_BROWSER_URL ?? "https://accounts.hss-science.org";

export const frontend = new FrontendApi(
  new Configuration({ basePath: KRATOS_PUBLIC_URL }),
);

export function getCookie(request: Request): string | undefined {
  return request.headers.get("cookie") ?? undefined;
}

export function initUrl(flow: string, returnTo?: string): string {
  const base = `${KRATOS_BROWSER_URL}/self-service/${flow}/browser`;
  if (returnTo !== undefined) {
    return `${base}?return_to=${encodeURIComponent(returnTo)}`;
  }
  return base;
}
