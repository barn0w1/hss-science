import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAccountsServiceLogin } from '@hss-science/api';
import { useAuth } from '../providers/AuthProvider';
import { STORAGE_KEY_REDIRECT_TO } from '../../../config/constants';

export const CallbackPage = () => {
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
            
            // The generated hook expects { data: V1LoginRequest }
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

                    // Clean URL (remove code)
                    window.history.replaceState({}, document.title, window.location.pathname);

                    if (redirectTo) {
                        try {
                            // Basic validation to ensure it's a valid URL or path
                            const url = new URL(redirectTo, window.location.origin);
                             // If exact match to current origin, use navigate (SPA navigation)
                             // otherwise use window.location.href (external or cross-app navigation)
                             if (url.origin === window.location.origin) {
                                  navigate(url.pathname + url.search + url.hash);
                             } else {
                                  window.location.href = redirectTo;
                             }
                        } catch (e) {
                             // Fallback if invalid URL
                             console.warn('Invalid redirect URL:', redirectTo);
                             navigate('/dashboard');
                        }
                    } else {
                        navigate('/dashboard');
                    }
                }
            }).catch((error) => {
                console.error("Login failed", error);
                navigate('/login?error=login_failed');
            });
        } else {
             // If no code, maybe just redirect to login
             navigate('/login');
        }
    }, [searchParams, navigate, login, loginMutation]);

    return (
        <div className="flex h-screen items-center justify-center bg-gray-100">
            <div className="text-xl font-semibold text-gray-700">Processing login...</div>
        </div>
    );
};
