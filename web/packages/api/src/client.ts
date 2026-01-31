import createClient, { type Client } from "openapi-fetch";

export type CreateClientOptions = Omit<Parameters<typeof createClient>[0], "baseUrl">;

export const createTypedClient = <TPaths>(
  baseUrl: string,
  init?: CreateClientOptions
): Client<TPaths> => {
  return createClient<TPaths>({
    baseUrl,
    ...(init ?? {}),
  });
};
