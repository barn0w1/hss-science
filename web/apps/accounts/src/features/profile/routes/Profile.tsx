import { useAccountsServiceGetMe } from '@hss-science/api';
import { useAuth } from '../../auth/hooks/useAuth';

export const Profile = () => {
    const { logout } = useAuth();
    const { data: user, isLoading, error } = useAccountsServiceGetMe();

    if (isLoading) {
        return (
            <div className="flex min-h-screen items-center justify-center bg-white">
                <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-gray-900"></div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="flex min-h-screen items-center justify-center bg-white">
                <div className="text-red-500">Failed to load account information.</div>
            </div>
        );
    }

    return (
        <div className="flex min-h-screen flex-col items-center bg-white pt-20">
            <div className="w-full max-w-md p-6">
                <div className="mb-8 text-center">
                    <h1 className="text-2xl font-normal text-gray-900">Account</h1>
                    <p className="mt-2 text-gray-500">Manage your account info</p>
                </div>

                <div className="mb-4 rounded-2xl border border-gray-200 bg-white p-8 text-center shadow-sm">
                    <div className="relative mb-4 inline-block">
                         {user?.avatar_url ? (
                            <img 
                                src={user.avatar_url} 
                                alt={user.name} 
                                className="h-24 w-24 rounded-full object-cover"
                            />
                        ) : (
                            <div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full bg-blue-600 text-3xl font-medium text-white">
                                {user?.name?.charAt(0).toUpperCase()}
                            </div>
                        )}
                    </div>
                    
                    <h2 className="mb-1 text-xl font-medium text-gray-900">{user?.name}</h2>
                    <p className="mb-6 inline-block rounded bg-gray-100 px-2 py-1 text-xs font-semibold uppercase tracking-wider text-gray-500">
                        {user?.role || 'User'}
                    </p>

                    <div className="mt-2 border-t border-gray-100 pt-6">
                        <div className="flex items-center justify-between text-sm">
                            <span className="text-gray-500">ID</span>
                            <span className="font-mono text-gray-700">{user?.id}</span>
                        </div>
                    </div>
                </div>

                <div className="mt-8 flex justify-center">
                    <button 
                        onClick={logout}
                        className="rounded border border-gray-300 px-6 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 md:w-auto"
                    >
                        Sign out
                    </button>
                </div>
            </div>
        </div>
    );
};
