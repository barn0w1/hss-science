import { useEffect, useState } from 'react';
import { fetchMe } from '../data/profile-api';
import { useAuth } from '../../auth/model/useAuth';

type User = Awaited<ReturnType<typeof fetchMe>>;

export const Profile = () => {
  const { logout } = useAuth();
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      try {
        const data = await fetchMe();
        if (mounted) setUser(data);
      } catch {
        if (mounted) setError('Failed to load account information.');
      } finally {
        if (mounted) setIsLoading(false);
      }
    };

    load();
    return () => {
      mounted = false;
    };
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-white text-sm text-gray-500">
        Loading...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-screen items-center justify-center bg-white text-sm text-red-600">
        {error}
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-white px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="space-y-1">
          <h1 className="text-xl font-medium text-gray-900">Account</h1>
          <p className="text-sm text-gray-500">{user?.name}</p>
        </div>

        <div className="rounded border border-gray-200 p-4 text-sm text-gray-700">
          <div className="flex justify-between">
            <span className="text-gray-500">ID</span>
            <span className="font-mono">{user?.id}</span>
          </div>
          <div className="mt-2 flex justify-between">
            <span className="text-gray-500">Role</span>
            <span>{user?.role ?? 'User'}</span>
          </div>
        </div>

        <button
          onClick={logout}
          className="w-full rounded border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
        >
          Sign out
        </button>
      </div>
    </div>
  );
};