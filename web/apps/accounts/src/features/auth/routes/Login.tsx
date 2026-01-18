import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAccountsServiceGetAuthUrl } from '@hss-science/api';
import { STORAGE_KEY_REDIRECT_TO } from '../../../utils/constants';

export const Login = () => {
    const [searchParams] = useSearchParams();
    
    // Save redirect_to
    useEffect(() => {
        const redirectTo = searchParams.get('redirect_to');
        if (redirectTo) {
            sessionStorage.setItem(STORAGE_KEY_REDIRECT_TO, redirectTo);
        }
    }, [searchParams]);

    // useQuery で GET リクエスト
    // refetch を手動で呼び出すために enabled: false
    const { refetch, isFetching } = useAccountsServiceGetAuthUrl({
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
        <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
            <div className="w-full max-w-sm rounded-xl border border-gray-100 bg-white p-8 text-center shadow-lg">
                <div className="mb-6 flex justify-center">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-indigo-100 text-indigo-600">
                        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" x2="3" y1="12" y2="12"/></svg>
                    </div>
                </div>
                
                <h1 className="mb-2 text-2xl font-bold text-gray-900">Sign in</h1>
                <p className="mb-8 text-sm text-gray-500">to continue to HSS Science</p>
                
                <button
                    onClick={handleLogin}
                    disabled={isFetching}
                    className={`flex w-full items-center justify-center gap-2 rounded-lg py-3 px-4 font-medium shadow-sm transition-all duration-200
                        ${isFetching 
                            ? 'cursor-not-allowed bg-gray-100 text-gray-400' 
                            : 'bg-[#5865F2] text-white hover:bg-[#4752C4] hover:shadow-md'
                        }`}
                >
                    {isFetching ? 'Redirecting...' : 'Sign in with Discord'}
                </button>
            </div>
        </div>
    );
};
