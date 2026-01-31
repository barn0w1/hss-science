import createClient, { type Client } from "openapi-fetch";

import type { paths } from "./generated/accounts";

export type { paths };
export type ApiClient = Client<paths>;

export const createApiClient = (
  baseUrl: string,
  init?: Omit<Parameters<typeof createClient>[0], "baseUrl">
): ApiClient => {
  return createClient<paths>({
    baseUrl,
    ...(init ?? {}),
  });
};
