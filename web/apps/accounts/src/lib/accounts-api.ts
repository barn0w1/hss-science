import { createAccountsClient } from "@hss-science/api";

const ACCOUNTS_API_BASE_URL =
  import.meta.env.VITE_ACCOUNTS_API_BASE_URL ?? "https://accounts.hss-science.org/api";

const client = createAccountsClient(ACCOUNTS_API_BASE_URL, {
  credentials: "include",
});

let accessToken: string | null = null;

export const setAccessToken = (token: string | null) => {
  accessToken = token;
};

export const getAccessToken = () => accessToken;

const withAuthHeaders = (headers?: Record<string, string>) => {
  if (!accessToken) return headers;
  return {
    ...(headers ?? {}),
    Authorization: `Bearer ${accessToken}`,
  };
};

const unwrap = async <T>(promise: Promise<{ data?: T; error?: unknown }>) => {
  const { data, error } = await promise;
  if (error) {
    throw error;
  }
  if (data === undefined) {
    throw new Error("No data");
  }
  return data;
};

export const fetchAuthUrl = () =>
  unwrap(client.GET("/v1/auth/url"));

export const loginWithCode = (code: string) =>
  unwrap(
    client.POST("/v1/auth/login", {
      body: { code },
    })
  );

export const refreshAccessToken = () =>
  unwrap(
    client.POST("/v1/auth/refresh", {
      body: {},
    })
  );

export const logout = () =>
  unwrap(
    client.POST("/v1/auth/logout", {
      body: {},
    })
  );

export const fetchMe = () =>
  unwrap(
    client.GET("/v1/users/me", {
      headers: withAuthHeaders(),
    })
  );
