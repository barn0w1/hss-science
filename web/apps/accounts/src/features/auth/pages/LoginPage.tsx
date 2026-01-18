import { useAccountsServiceGetAuthUrl } from '@hss-science/api';

export const LoginPage = () => {
    const { refetch, isFetching } = useAccountsServiceGetAuthUrl({
        redirect_url: window.location.origin + '/callback'
    }, {
        query: {
            enabled: false
        }
    });

    const handleLogin = async () => {
        const result = await refetch();
        if (result.data?.url) {
            window.location.href = result.data.url;
        }
    };

    return (
        <div className="flex h-screen items-center justify-center bg-gray-100">
            <div className="w-full max-w-md bg-white p-8 rounded-lg shadow-md text-center">
                <h1 className="text-2xl font-bold mb-6 text-gray-800">Welcome</h1>
                <p className="mb-8 text-gray-600">Please sign in to continue.</p>
                <button
                    onClick={handleLogin}
                    disabled={isFetching}
                    className="w-full bg-indigo-600 text-white py-2 px-4 rounded hover:bg-indigo-700 disabled:opacity-50 transition-colors"
                >
                    {isFetching ? 'Redirecting...' : 'Login with Discord'}
                </button>
            </div>
        </div>
    );
};
