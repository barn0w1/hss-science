import { useAuth } from '../../auth/providers/AuthProvider';
import { useAccountsServiceGetMe } from '@hss-science/api';

export const DashboardPage = () => {
    const { logout } = useAuth();
    const { data: user, isLoading, error } = useAccountsServiceGetMe();

    return (
        <div className="min-h-screen bg-gray-50 p-8">
            <div className="max-w-4xl mx-auto bg-white p-8 rounded-xl shadow-sm border border-gray-100">
                <div className="flex justify-between items-center mb-8">
                    <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
                    <button 
                        onClick={logout}
                        className="bg-white text-gray-700 border border-gray-200 py-2 px-4 rounded-lg hover:bg-gray-50 hover:text-red-600 hover:border-red-200 transition-colors text-sm font-medium"
                    >
                        Logout
                    </button>
                </div>

                {isLoading ? (
                    <div className="text-gray-500 animate-pulse">Loading user profile...</div>
                ) : error ? (
                    <div className="p-4 bg-red-50 text-red-700 rounded-lg">
                        Failed to load user profile.
                    </div>
                ) : (
                    <div className="flex items-center space-x-6 p-6 bg-gray-50 rounded-lg">
                        {user?.avatar_url ? (
                            <img 
                                src={user.avatar_url} 
                                alt={`${user.name}'s avatar`} 
                                className="w-20 h-20 rounded-full border-4 border-white shadow-sm"
                            />
                        ) : (
                            <div className="w-20 h-20 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-500 text-2xl font-bold border-4 border-white shadow-sm">
                                {user?.name?.charAt(0) || 'U'}
                            </div>
                        )}
                        <div>
                            <h2 className="text-2xl font-bold text-gray-900">{user?.name}</h2>
                            <p className="text-gray-500">{user?.role}</p>
                            <div className="mt-2 text-xs text-gray-400 font-mono">ID: {user?.id}</div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};
