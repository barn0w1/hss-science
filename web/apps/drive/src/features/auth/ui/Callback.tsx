import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { loginWithCode } from '../data/auth-api';
import { useAuth } from '../model/useAuth';
import { STORAGE_KEY_REDIRECT_TO } from '../../../utils/constants';

export const Callback = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { login } = useAuth();
  const hasRun = useRef(false);

  useEffect(() => {
    if (hasRun.current) return;

    const code = searchParams.get('code');
    if (!code) {
      navigate('/login');
      return;
    }

    hasRun.current = true;

    loginWithCode(code)
      .then((response) => {
        if (response.access_token) {
          login(response.access_token);

          const redirectTo = sessionStorage.getItem(STORAGE_KEY_REDIRECT_TO);
          sessionStorage.removeItem(STORAGE_KEY_REDIRECT_TO);
          window.history.replaceState({}, document.title, window.location.pathname);

          if (redirectTo) {
            try {
              const url = new URL(redirectTo, window.location.origin);
              if (url.origin === window.location.origin) {
                navigate(url.pathname + url.search + url.hash);
              } else {
                window.location.href = redirectTo;
              }
            } catch (error) {
              console.warn('Invalid redirect URL:', redirectTo, error);
              navigate('/');
            }
          } else {
            navigate('/');
          }
        }
      })
      .catch((error) => {
        console.error('Login failed', error);
        navigate('/login');
      });
  }, [searchParams, navigate, login]);

  return (
    <div className="flex h-screen items-center justify-center bg-white text-sm text-gray-500">
      Authenticating...
    </div>
  );
};