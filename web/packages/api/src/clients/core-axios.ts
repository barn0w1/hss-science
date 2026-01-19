import axios from 'axios';

// Axios Instance
export const AXIOS_INSTANCE = axios.create();

// Common request handling
AXIOS_INSTANCE.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Common response handling
AXIOS_INSTANCE.interceptors.response.use(
  (response) => response,
  (error) => {
    // Handle logout on 401 errors here
    return Promise.reject(error);
  }
);