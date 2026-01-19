import axios from 'axios';

// In memory for access token
let accessToken: string | null = null;

export const setAccessToken = (token: string) => {
  accessToken = token;
};

export const AXIOS_INSTANCE = axios.create({
  withCredentials: true, // send Cookie(refresh token)
});

// Attach access token to headers on each request
AXIOS_INSTANCE.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`;
  }
  return config;
});

// Automatic refresh on 401 errors
AXIOS_INSTANCE.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // 401エラー かつ まだリトライしていない場合
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      try {
        const { data } = await axios.post(
          `${originalRequest.baseURL}/v1/auth/refresh`,
          {}, 
          { withCredentials: true } // send Cookie(refresh token)
        );

        // 新しいアクセストークンを保存
        if (data.access_token) {
            setAccessToken(data.access_token);
            // 元のリクエストのヘッダーを書き換えて再送
            originalRequest.headers.Authorization = `Bearer ${data.access_token}`;
            return AXIOS_INSTANCE(originalRequest);
        }
      } catch (refreshError) {
        // リフレッシュ失敗時はログアウト扱い（ログイン画面へ遷移など）
        return Promise.reject(refreshError);
      }
    }
    return Promise.reject(error);
  }
);