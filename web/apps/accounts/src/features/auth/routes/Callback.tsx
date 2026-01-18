import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAccountsServiceLogin } from '@hss-science/api';
import { useAuth } from '../hooks/useAuth';
import { STORAGE_KEY_REDIRECT_TO } from '../../../utils/constants';

export const Callback = () => {
    const [searchParams] = useSearchParams();
    const navigate = useNavigate();
    const { login } = useAuth();
    const loginMutation = useAccountsServiceLogin();
    const hasRun = useRef(false);

    useEffect(() => {
        if (hasRun.current) return;
        
        const code = searchParams.get('code');
        
        if (code) {
            hasRun.current = true;
            
            loginMutation.mutateAsync({
                data: {
                    code
                }
            }).then((response) => {
                if (response.access_token) {
                    login(response.access_token);
                    
                    // Handle redirection
                    const redirectTo = sessionStorage.getItem(STORAGE_KEY_REDIRECT_TO);
                    sessionStorage.removeItem(STORAGE_KEY_REDIRECT_TO);

                    // Clean URL
                    window.history.replaceState({}, document.title, window.location.pathname);

                    if (redirectTo) {
                        try {
                            const url = new URL(redirectTo, window.location.origin);
                             // 同一オリジンならSPA遷移
                             if (url.origin === window.location.origin) {
                                  navigate(url.pathname + url.search + url.hash);
                             } else {
                                  window.location.href = redirectTo;
                             }
                        } catch (e) {
                             console.warn('Invalid redirect URL:', redirectTo);
                             navigate('/');
                        }
                    } else {
                        navigate('/');
                    }
                }
            }).catch((error) => {
                console.error("Login failed", error);
                navigate('/login');
            });
        } else {
             navigate('/login');
        }
    }, [searchParams, navigate, login, loginMutation]);

    return (
        <div className="flex h-screen items-center justify-center bg-gray-50">
            <div className="flex flex-col items-center gap-4">
                <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-indigo-600"></div>
                <div className="text-sm font-medium text-gray-600">Authenticating...</div>
            </div>
        </div>
    );
};
