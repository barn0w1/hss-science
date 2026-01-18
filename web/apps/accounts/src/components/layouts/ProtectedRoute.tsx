import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useAuth } from '../../features/auth/hooks/useAuth';

export const ProtectedRoute = () => {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    // 認証チェック中は何も表示しない、またはローディングを表示
    // ここで Navigate させないことが超重要
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-gray-50">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-indigo-600"></div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // 認証されていない場合はログイン画面へ
    // redirect_to を付与しても良いが、今回はLoginページ側でパラメータを処理するので
    // ここで単純にリダイレクトさせる
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return <Outlet />;
};
