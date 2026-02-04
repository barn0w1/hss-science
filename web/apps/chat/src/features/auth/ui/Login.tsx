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
    <div
      className="
        flex min-h-screen items-center justify-center
        bg-gradient-to-b from-[#f8f7f4] to-[#f4f3ef]
        px-4 sm:px-6
        font-sans
      "
    >
      <div className="w-full max-w-sm rounded-md bg-[#fdfcf8] p-8 shadow-[0_20px_40px_rgba(0,0,0,0.08)]">
        {/* Header */}
        <div className="mb-6 space-y-1 text-center">
          <h1 className="font-serif text-xl font-semibold tracking-wide text-gray-900">
            Sign in
          </h1>
          <p className="text-sm text-gray-500">
            Continue with your account
          </p>
        </div>

        {/* SSO Button */}
        <button
          onClick={handleLogin}
          disabled={isLoading}
          className="
            w-full rounded
            border border-gray-300
            bg-white
            px-4 py-2.5
            text-sm font-medium text-gray-800
            shadow-sm
            transition
            hover:bg-gray-50
            active:translate-y-[1px]
            disabled:cursor-not-allowed
            disabled:opacity-60
          "
        >
          {isLoading ? 'Redirecting…' : 'Continue with Discord'}
        </button>
      </div>
    </div>
  );
};
