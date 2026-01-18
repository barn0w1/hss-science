import axios from 'axios';
import type { AxiosRequestConfig } from 'axios';


// GatewayのURL (環境変数で切り替えられるようにするのがベストだが、一旦直書き)
export const AXIOS_INSTANCE = axios.create({
  baseURL: 'https://accounts.hss-science.org/api', // Gateway Port
  withCredentials: true, // Required for HttpOnly Cookies
  headers: {
    'Content-Type': 'application/json',
  },
});

export const setAccessToken = (token: string | null) => {
  if (token) {
    AXIOS_INSTANCE.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  } else {
    delete AXIOS_INSTANCE.defaults.headers.common['Authorization'];
  }
};

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