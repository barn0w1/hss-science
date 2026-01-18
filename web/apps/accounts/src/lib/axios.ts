import { AXIOS_INSTANCE } from '@hss-science/api';

export const setAccessToken = (token: string | null) => {
  if (token) {
    AXIOS_INSTANCE.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  } else {
    delete AXIOS_INSTANCE.defaults.headers.common['Authorization'];
  }
};

export { AXIOS_INSTANCE };
