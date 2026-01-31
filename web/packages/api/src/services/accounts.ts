import type { Client } from "openapi-fetch";

import type { paths as AccountsPaths } from "../generated/accounts";
import { createTypedClient, type CreateClientOptions } from "../client";

export type { AccountsPaths };
export type AccountsClient = Client<AccountsPaths>;

export const createAccountsClient = (
  baseUrl: string,
  init?: CreateClientOptions
): AccountsClient => {
  return createTypedClient<AccountsPaths>(baseUrl, init);
};

// Backward-compatible names
export type ApiClient = AccountsClient;
export type paths = AccountsPaths;
export const createApiClient = createAccountsClient;
