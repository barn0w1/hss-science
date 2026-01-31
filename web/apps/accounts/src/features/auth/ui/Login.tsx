import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { fetchAuthUrl } from '../data/auth-api';
import { STORAGE_KEY_REDIRECT_TO } from '../../../utils/constants';

export const Login = () => {
  const [searchParams] = useSearchParams();
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    const redirectTo = searchParams.get('redirect_to');
    if (redirectTo) {
      sessionStorage.setItem(STORAGE_KEY_REDIRECT_TO, redirectTo);
    }
  }, [searchParams]);

  const handleLogin = async () => {
    setIsLoading(true);
    try {
      const result = await fetchAuthUrl();
      if (result.url) {
        window.location.href = result.url;
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-white px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="space-y-1 text-center">
          <h1 className="text-xl font-medium text-gray-900">Sign in</h1>
          <p className="text-sm text-gray-500">Continue with your account</p>
        </div>

        <button
          onClick={handleLogin}
          disabled={isLoading}
          className="w-full rounded border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-60"
        >
          {isLoading ? 'Redirecting...' : 'Continue with Discord'}
        </button>
      </div>
    </div>
  );
};