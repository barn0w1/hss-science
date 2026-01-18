import { useAuth } from '../../auth/providers/AuthProvider';
import { useAccountsServiceGetMe } from '@hss-science/api';

export const DashboardPage = () => {
    const { logout } = useAuth();
    const { data: user, isLoading, error } = useAccountsServiceGetMe();

    if (isLoading) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-white">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-white">
                <div className="text-red-500">Failed to load account information.</div>
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-white flex flex-col items-center pt-20">
            <div className="w-full max-w-md p-6">
                <div className="text-center mb-8">
                    <h1 className="text-2xl font-normal text-gray-900">Account</h1>
                    <p className="text-gray-500 mt-2">Manage your account info</p>
                </div>

                <div className="bg-white border text-center border-gray-200 rounded-2xl p-8 shadow-sm">
                    <div className="relative inline-block mb-4">
                         {user?.avatar_url ? (
                            <img 
                                src={user.avatar_url} 
                                alt={user.name} 
                                className="w-24 h-24 rounded-full object-cover"
                            />
                        ) : (
                            <div className="w-24 h-24 rounded-full bg-blue-600 flex items-center justify-center text-white text-3xl font-medium mx-auto">
                                {user?.name?.charAt(0).toUpperCase()}
                            </div>
                        )}
                    </div>
                    
                    <h2 className="text-xl font-medium text-gray-900 mb-1">{user?.name}</h2>
                    <p className="text-gray-500 text-sm mb-6 uppercase tracking-wider text-xs font-semibold bg-gray-100 inline-block px-2 py-1 rounded">
                        {user?.role || 'User'}
                    </p>

                    <div className="border-t border-gray-100 pt-6 mt-2">
                        <div className="flex justify-between items-center text-sm">
                            <span className="text-gray-500">ID</span>
                            <span className="font-mono text-gray-700">{user?.id}</span>
                        </div>
                    </div>
                </div>

                <div className="mt-8 flex justify-center">
                    <button 
                        onClick={logout}
                        className="text-gray-600 hover:text-gray-900 font-medium text-sm border border-gray-300 px-6 py-2 rounded md:w-auto hover:bg-gray-50 transition-colors"
                    >
                        Sign out
                    </button>
                </div>
            </div>
        </div>
    );
};
