import { Configuration, FrontendApi } from "@ory/kratos-client-fetch";

const KRATOS_INTERNAL_URL =
  process.env.KRATOS_INTERNAL_URL ??
  "http://kratos-public.identity.svc.cluster.local";

export const KRATOS_PUBLIC_URL =
  process.env.KRATOS_PUBLIC_URL ?? "https://accounts.hss-science.org";

export const frontend = new FrontendApi(
  new Configuration({ basePath: KRATOS_INTERNAL_URL }),
);

export function getCookie(request: Request): string | undefined {
  return request.headers.get("cookie") ?? undefined;
}

export function initUrl(flow: string, returnTo?: string): string {
  const base = `${KRATOS_PUBLIC_URL}/self-service/${flow}/browser`;
  if (returnTo !== undefined) {
    return `${base}?return_to=${encodeURIComponent(returnTo)}`;
  }
  return base;
}
