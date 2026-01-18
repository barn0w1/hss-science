import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAccountsServiceGetAuthUrl } from '@hss-science/api';
import { STORAGE_KEY_REDIRECT_TO } from '../../../config/constants';

export const LoginPage = () => {
    const [searchParams] = useSearchParams();
    
    // Save redirect_to before anything else
    useEffect(() => {
        const redirectTo = searchParams.get('redirect_to');
        if (redirectTo) {
            sessionStorage.setItem(STORAGE_KEY_REDIRECT_TO, redirectTo);
        }
    }, [searchParams]);

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
        <div className="min-h-screen flex flex-col items-center justify-center bg-gray-50 p-4">
            <div className="w-full max-w-sm bg-white p-8 rounded-xl shadow-lg text-center border border-gray-100">
                <div className="mb-6 flex justify-center">
                    <div className="h-12 w-12 bg-indigo-100 rounded-full flex items-center justify-center text-indigo-600">
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-log-in"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" x2="3" y1="12" y2="12"/></svg>
                    </div>
                </div>
                
                <h1 className="text-2xl font-bold mb-2 text-gray-900">Welcome Back</h1>
                <p className="mb-8 text-gray-500 text-sm">Sign in to access your dashboard</p>
                
                <button
                    onClick={handleLogin}
                    disabled={isFetching}
                    className={`w-full py-3 px-4 rounded-lg font-medium shadow-sm transition-all duration-200 flex items-center justify-center gap-2
                        ${isFetching 
                            ? 'bg-gray-100 text-gray-400 cursor-not-allowed' 
                            : 'bg-[#5865F2] hover:bg-[#4752C4] text-white hover:shadow-md'
                        }`}
                >
                    {isFetching ? (
                        <>
                            <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-gray-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            Redirecting...
                        </>
                    ) : (
                        'Login with Discord'
                    )}
                </button>
            </div>
        </div>
    );
};
