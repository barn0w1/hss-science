import axios, { AxiosRequestConfig } from 'axios';

// GatewayのURL (環境変数で切り替えられるようにするのがベストだが、一旦直書き)
export const AXIOS_INSTANCE = axios.create({
  baseURL: 'http://localhost:3000', // Gateway Port
  headers: {
    'Content-Type': 'application/json',
  },
});

// Orvalが使用するカスタム関数
export const customInstance = <T>(
  config: AxiosRequestConfig,
  options?: AxiosRequestConfig,
): Promise<T> => {
  const source = axios.CancelToken.source();
  const promise = AXIOS_INSTANCE({
    ...config,
    ...options,
    cancelToken: source.token,
  }).then(({ data }) => data);

  // @ts-ignore
  promise.cancel = () => {
    source.cancel('Query was cancelled');
  };

  return promise;
};