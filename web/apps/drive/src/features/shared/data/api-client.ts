import { createAccountsClient } from "@hss-science/api";

const ACCOUNTS_API_BASE_URL =
  import.meta.env.VITE_ACCOUNTS_API_BASE_URL ?? "https://accounts.hss-science.org/api";

export const accountsClient = createAccountsClient(ACCOUNTS_API_BASE_URL, {
  credentials: "include",
});