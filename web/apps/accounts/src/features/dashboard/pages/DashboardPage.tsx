import { useAuth } from '../../auth/providers/AuthProvider';

export const DashboardPage = () => {
    const { logout } = useAuth();

    return (
        <div className="min-h-screen bg-gray-50 p-8">
            <div className="max-w-4xl mx-auto bg-white p-6 rounded-lg shadow-sm">
                <h1 className="text-3xl font-bold mb-4 text-gray-800">Dashboard</h1>
                <p className="mb-6 text-gray-600">Welcome to the protected dashboard! You are successfully authenticated.</p>
                <button 
                    onClick={logout}
                    className="bg-red-500 text-white py-2 px-4 rounded hover:bg-red-600 transition-colors"
                >
                    Logout
                </button>
            </div>
        </div>
    );
};
