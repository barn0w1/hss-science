import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAccountsServiceLogin } from '@hss-science/api';
import { useAuth } from '../providers/AuthProvider';

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
                    navigate('/dashboard');
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
