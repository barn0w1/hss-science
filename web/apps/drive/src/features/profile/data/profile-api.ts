import { accountsClient } from '../../shared/data/api-client';
import { getAccessToken } from '../../auth/data/token-store';

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

const withAuthHeaders = () => {
  const token = getAccessToken();
  if (!token) return undefined;
  return {
    Authorization: `Bearer ${token}`,
  };
};

export const fetchMe = () =>
  unwrap(
    accountsClient.GET('/v1/users/me', {
      headers: withAuthHeaders(),
    })
  );