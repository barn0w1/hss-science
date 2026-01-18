import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAccountsServiceGetAuthUrl } from '@hss-science/api';
import { AuthLayout } from '../../../components/layouts/AuthLayout';
import { Button } from '../../../components/ui/Button';
import { STORAGE_KEY_REDIRECT_TO } from '../../../utils/constants';

const DiscordIcon = () => (
    <svg className="w-5 h-5 transition-colors" fill="currentColor" viewBox="0 0 24 24">
        <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994.021-.041-.001-.09-.045-.106a12.99 12.99 0 0 1-1.864-.895.077.077 0 0 1-.007-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.896.075.075 0 0 0-.044.106c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
    </svg>
);

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
        <AuthLayout 
            title="Sign in"
            subtitle="HSS Science"
        >
            <div className="space-y-5">
                <Button
                    onClick={handleLogin}
                    isLoading={isFetching}
                    className="w-full h-11 bg-slate-900 hover:bg-slate-800 text-white border-transparent shadow-sm"
                    leftIcon={<DiscordIcon />}
                >
                    {isFetching ? 'Connecting to Discord...' : 'Continue with Discord'}
                </Button>

                <p className="text-[11px] text-slate-400 text-center">
                    Single Sign-On for *.hss-science.org
                </p>
            </div>
        </AuthLayout>
    );
};
