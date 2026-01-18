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
        <div className="min-h-screen flex flex-col items-center justify-center bg-white p-4">
            <div className="w-full max-w-sm text-center">
                <h1 className="text-2xl font-medium text-gray-900 mb-2">Sign in</h1>
                <p className="mb-8 text-gray-500">to continue to HSS Science</p>
                
                <button
                    onClick={handleLogin}
                    disabled={isFetching}
                    className={`w-full py-2.5 px-4 rounded border border-gray-300 font-medium text-gray-700 bg-white hover:bg-gray-50 transition-colors flex items-center justify-center gap-3 ${isFetching ? 'opacity-50 cursor-not-allowed' : ''}`}
                >
                    <svg className="w-5 h-5 text-[#5865F2]" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M20.317 4.3698a19.7913 19.7913 0 00-4.8851-1.5152.0741.0741 0 00-.0785.0371c-.211.3753-.4447.8648-.6083 1.2495-1.8447-.2762-3.68-.2762-5.4868 0-.1636-.3933-.4058-.8742-.6177-1.2495a.077.077 0 00-.0785-.037 19.7363 19.7363 0 00-4.8852 1.515.0699.0699 0 00-.0321.0277C.5334 9.0458-.319 13.5799.0992 18.0578a.0824.0824 0 00.0312.0561c2.0528 1.5076 4.0413 2.4228 5.9929 3.0294a.0777.0777 0 00.0842-.0276c.4616-.6304.8731-1.2952 1.226-1.9942a.076.076 0 00-.0416-.1057c-.6528-.2476-1.2743-.5495-1.8722-.8923a.077.077 0 01-.0076-.1277c.1258-.0943.2517-.1892.3718-.2914a.0743.0743 0 01.0776-.0105c3.9278 1.7933 8.18 1.7933 12.0614 0a.0739.0739 0 01.0785.0095c.1202.1023.246.1981.3728.2924a.077.077 0 01-.0066.1276 12.2986 12.2986 0 01-1.873.8914.0766.0766 0 00-.0407.1067c.3604.698.7719 1.3628 1.225 1.9932a.076.076 0 00.0842.0286c1.961-.6067 3.9495-1.5219 6.0023-3.0294a.077.077 0 00.0313-.0552c.5004-5.177-.8382-9.6739-3.5485-13.6604a.061.061 0 00-.0312-.0286zM8.02 15.3312c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9555-2.4189 2.157-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.946 2.419-2.1569 2.419zm7.9748 0c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9554-2.4189 2.1569-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.946 2.419-2.1568 2.419z"/></svg>
                    <span>{isFetching ? 'Redirecting...' : 'Sign in with Discord'}</span>
                </button>
            </div>
        </div>
    );
};
