import { accountsClient } from '../../shared/data/api-client';

const unwrap = async <T>(promise: Promise<{ data?: T; error?: unknown }>) => {
  const { data, error } = await promise;
  if (error) {
    throw error;
  }
  if (data === undefined) {
    throw new Error('No data');
  }
  return data;
};

export const fetchAuthUrl = () => unwrap(accountsClient.GET('/v1/auth/url'));

export const loginWithCode = (code: string) =>
  unwrap(
    accountsClient.POST('/v1/auth/login', {
      body: { code },
    })
  );

export const refreshAccessToken = () =>
  unwrap(
    accountsClient.POST('/v1/auth/refresh', {
      body: {},
    })
  );

export const logout = () =>
  unwrap(
    accountsClient.POST('/v1/auth/logout', {
      body: {},
    })
  );