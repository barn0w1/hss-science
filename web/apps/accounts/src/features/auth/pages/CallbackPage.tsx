import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAccountsServiceLogin } from '@hss-science/api'; // ★自動生成コード

export const CallbackPage = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const code = searchParams.get('code');

  // Orvalが生成したMutation Hook
  const { mutate: login, isPending, error } = useAccountsServiceLogin();

  useEffect(() => {
    if (!code) {
      navigate('/login'); // コードがなければログイン画面へ
      return;
    }

    // Backendの Login API を叩く
    login(
      { data: { code } }, 
      {
        onSuccess: (data) => {
          console.log('Login Success:', data);
          // Token保存 (本来は専用のstorage utilityを作るべきだが一旦直書き)
          if (data.accessToken) {
            localStorage.setItem('accessToken', data.accessToken);
          }
          if (data.refreshToken) {
            localStorage.setItem('refreshToken', data.refreshToken);
          }
          
          // ダッシュボードへ
          navigate('/');
        },
        onError: (err) => {
          console.error('Login Failed:', err);
        }
      }
    );
  }, [code, login, navigate]);

  if (isPending) {
    return <div className="p-10 text-center">Logging in...</div>;
  }

  if (error) {
    return <div className="p-10 text-center text-red-500">Login Failed. Please try again.</div>;
  }

  return null;
};