import { AxiosRequestConfig } from 'axios';
import { AXIOS_INSTANCE } from './core-axios';

// Base URL
const ACCOUNTS_BASE_URL = process.env.VITE_API_ACCOUNTS_URL || 'https://accounts.hss-science.org/api';

// Service client factory
const createClient = (baseURL: string) => {
  return <T>(config: AxiosRequestConfig, options?: AxiosRequestConfig): Promise<T> => {
    return AXIOS_INSTANCE({
      ...config,
      ...options,
      baseURL,
    }).then(({ data }) => data);
  };
};

// Clients
export const accountsClient = createClient(ACCOUNTS_BASE_URL);